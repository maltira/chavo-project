import { ApiError, request, setSessionLostHandler } from './index'
import { getAccessToken, setAccessToken } from './token'

function reply(status: number, body?: unknown) {
    return new Response(body === undefined ? null : JSON.stringify(body), { status })
}

describe('request', () => {
    const fetchMock = vi.fn()

    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
        setAccessToken(null)
        setSessionLostHandler(null)
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('sends JSON with the access token and parses the response', async () => {
        setAccessToken('token-1')
        fetchMock.mockResolvedValueOnce(reply(200, { ok: true }))

        const data = await request<{ ok: boolean }>('/users/me', { method: 'POST', body: { a: 1 } })

        expect(data).toEqual({ ok: true })
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me')
        expect(init.headers.Authorization).toBe('Bearer token-1')
        expect(init.headers['Content-Type']).toBe('application/json')
        expect(init.body).toBe('{"a":1}')
        expect(init.credentials).toBe('same-origin')
    })

    it('throws ApiError with status, reason and message', async () => {
        fetchMock.mockResolvedValueOnce(
            reply(401, { error: 'Неверная почта или пароль', reason: 'INVALID_CREDENTIALS' }),
        )

        const error = await request('/auth/login', { method: 'POST', body: {} }).catch((e) => e)

        expect(error).toBeInstanceOf(ApiError)
        expect(error).toMatchObject({
            status: 401,
            reason: 'INVALID_CREDENTIALS',
            message: 'Неверная почта или пароль',
        })
    })

    it('does not try to refresh on 401 from /auth/*', async () => {
        fetchMock.mockResolvedValueOnce(reply(401, { error: 'x', reason: 'INVALID_CREDENTIALS' }))

        await expect(request('/auth/login', { method: 'POST', body: {} })).rejects.toBeInstanceOf(
            ApiError,
        )
        expect(fetchMock).toHaveBeenCalledTimes(1)
    })

    it('refreshes the token on 401 and repeats the request', async () => {
        setAccessToken('old')
        fetchMock
            .mockResolvedValueOnce(reply(401, { error: 'x', reason: 'UNAUTHORIZED' }))
            .mockResolvedValueOnce(reply(200, { access_token: 'new' }))
            .mockResolvedValueOnce(reply(200, { id: 1 }))

        const data = await request<{ id: number }>('/users/me')

        expect(data).toEqual({ id: 1 })
        expect(getAccessToken()).toBe('new')
        expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/auth/refresh')
        expect(fetchMock.mock.calls[2]?.[1].headers.Authorization).toBe('Bearer new')
    })

    it('shares one refresh between parallel requests', async () => {
        setAccessToken('old')
        fetchMock.mockImplementation(async (url: string, init: RequestInit) => {
            if (url === '/api/auth/refresh') {
                return reply(200, { access_token: 'new' })
            }
            const headers = init.headers as Record<string, string>
            return headers.Authorization === 'Bearer new' ? reply(200, { ok: 1 }) : reply(401, {})
        })

        await Promise.all([request('/a'), request('/b'), request('/c')])

        const refreshes = fetchMock.mock.calls.filter(([url]) => url === '/api/auth/refresh')
        expect(refreshes).toHaveLength(1)
    })

    it('drops the session when the server rejects the refresh', async () => {
        const onLost = vi.fn()
        setSessionLostHandler(onLost)
        setAccessToken('old')
        fetchMock
            .mockResolvedValueOnce(reply(401, { error: 'x', reason: 'UNAUTHORIZED' }))
            .mockResolvedValueOnce(reply(401, {}))

        await expect(request('/users/me')).rejects.toBeInstanceOf(ApiError)

        expect(getAccessToken()).toBeNull()
        expect(onLost).toHaveBeenCalledTimes(1)
    })

    it('keeps the session when refresh cannot reach the server', async () => {
        const onLost = vi.fn()
        setSessionLostHandler(onLost)
        setAccessToken('old')
        fetchMock
            .mockResolvedValueOnce(reply(401, { error: 'x', reason: 'UNAUTHORIZED' }))
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))

        const error = await request('/users/me').catch((e) => e)

        expect(error).toMatchObject({ status: 0, reason: 'NETWORK_ERROR' })
        expect(getAccessToken()).toBe('old')
        expect(onLost).not.toHaveBeenCalled()
    })

    it('keeps the session when refresh fails with a server error', async () => {
        const onLost = vi.fn()
        setSessionLostHandler(onLost)
        setAccessToken('old')
        fetchMock
            .mockResolvedValueOnce(reply(401, { error: 'x', reason: 'UNAUTHORIZED' }))
            .mockResolvedValueOnce(reply(503, {}))

        await expect(request('/users/me')).rejects.toMatchObject({ reason: 'NETWORK_ERROR' })

        expect(getAccessToken()).toBe('old')
        expect(onLost).not.toHaveBeenCalled()
    })

    it('turns a network failure of a plain request into ApiError', async () => {
        fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'))

        await expect(request('/auth/login', { method: 'POST', body: {} })).rejects.toMatchObject({
            status: 0,
            reason: 'NETWORK_ERROR',
        })
    })
})
