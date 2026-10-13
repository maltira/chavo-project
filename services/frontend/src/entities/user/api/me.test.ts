import { ApiError } from '@/shared/api'

import { getMe } from './me'

describe('getMe', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('returns the profile', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(new Response(JSON.stringify({ username: 'me' }))),
        )
        await expect(getMe()).resolves.toMatchObject({ username: 'me' })
    })

    it('returns null when the profile is not created yet', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(new Response('{"error":"x"}', { status: 404 })),
        )
        await expect(getMe()).resolves.toBeNull()
    })

    it('passes other errors through', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 500 })))
        await expect(getMe()).rejects.toBeInstanceOf(ApiError)
    })
})
