import { getAccessToken, setAccessToken } from '@/shared/api'

import { bootstrapSession, useSession } from './store'

describe('session', () => {
    beforeEach(() => {
        setAccessToken(null)
        useSession.setState({ status: 'unknown' })
    })

    afterEach(() => {
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

    it('shows the login without dropping the cookie when the network is down', async () => {
        vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
        await bootstrapSession()
        expect(useSession.getState().status).toBe('guest')
    })
})
