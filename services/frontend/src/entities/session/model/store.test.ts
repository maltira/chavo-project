import { getAccessToken, setAccessToken } from '@/shared/api'

import { bootstrapSession, useSession } from './store'

describe('session', () => {
    beforeEach(() => {
        setAccessToken(null)
        useSession.setState({ status: 'unknown', unreachable: null })
    })

    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    it('starts unknown and signs in with a token', () => {
        expect(useSession.getState().status).toBe('unknown')
        useSession.getState().signIn('abc')
        expect(useSession.getState().status).toBe('authenticated')
        expect(getAccessToken()).toBe('abc')
    })

    it('signs out and forgets the token', () => {
        useSession.getState().signIn('abc')
        useSession.getState().signOut()
        expect(useSession.getState().status).toBe('guest')
        expect(getAccessToken()).toBeNull()
    })

    it('restores the session from the refresh cookie on start', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(new Response(JSON.stringify({ access_token: 'fresh' }))),
        )
        await bootstrapSession()
        expect(useSession.getState().status).toBe('authenticated')
        expect(getAccessToken()).toBe('fresh')
    })

    it('becomes a guest when there is no valid refresh cookie', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
        await bootstrapSession()
        expect(useSession.getState().status).toBe('guest')
    })

    it('waits on the splash and retries instead of signing out when the server is unreachable', async () => {
        vi.useFakeTimers()
        const fetchMock = vi
            .fn()
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'back' })))
        vi.stubGlobal('fetch', fetchMock)

        await bootstrapSession()
        // не выход: сессия может быть жива, просто нет связи
        expect(useSession.getState()).toMatchObject({ status: 'unknown', unreachable: 'server' })

        await vi.advanceTimersByTimeAsync(2_000)
        expect(fetchMock).toHaveBeenCalledTimes(2)
        expect(useSession.getState()).toMatchObject({ status: 'authenticated', unreachable: null })
    })

    it('says there is no internet and retries as soon as it is back', async () => {
        vi.useFakeTimers()
        vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
        const fetchMock = vi
            .fn()
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce(new Response('{}', { status: 401 }))
        vi.stubGlobal('fetch', fetchMock)

        await bootstrapSession()
        expect(useSession.getState().unreachable).toBe('offline')

        // событие online - повтор сразу, без ожидания таймера
        window.dispatchEvent(new Event('online'))
        await vi.advanceTimersByTimeAsync(0)
        expect(fetchMock).toHaveBeenCalledTimes(2)
        expect(useSession.getState()).toMatchObject({ status: 'guest', unreachable: null })
    })
})
