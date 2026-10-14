import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'

import { useSession } from '@/entities/session'
import { useNotifications } from '@/shared/lib'

import { applyEvent, presenceKey, RealtimeBridge } from './RealtimeBridge'
import type { Presence } from './RealtimeBridge'

class FakeSocket {
    static OPEN = 1
    static instances: FakeSocket[] = []
    readyState = 1
    onmessage: ((event: { data: string }) => void) | null = null
    onclose: ((event: { code: number }) => void) | null = null
    constructor(public url: string) {
        FakeSocket.instances.push(this)
    }
    send() {}
    close() {}
}

describe('applyEvent', () => {
    it('stores online and offline presence in the cache', () => {
        const client = new QueryClient()
        applyEvent({ type: 'user.online', data: { user_id: 'u1' } }, client)
        expect(client.getQueryData<Presence>(presenceKey('u1'))).toEqual({
            online: true,
            lastSeenAt: null,
        })

        applyEvent(
            { type: 'user.offline', data: { user_id: 'u1', last_seen_at: '2026-10-14T10:00:00Z' } },
            client,
        )
        expect(client.getQueryData<Presence>(presenceKey('u1'))).toEqual({
            online: false,
            lastSeenAt: '2026-10-14T10:00:00Z',
        })
    })

    it('ignores events it does not know yet', () => {
        const client = new QueryClient()
        applyEvent({ type: 'message.created', data: { conversation_id: 'c1' } }, client)
        expect(client.getQueryCache().getAll()).toHaveLength(0)
    })
})

describe('RealtimeBridge', () => {
    function mount(client = new QueryClient()) {
        const view = render(
            <QueryClientProvider client={client}>
                <RealtimeBridge />
            </QueryClientProvider>,
        )
        const socket = () => FakeSocket.instances[FakeSocket.instances.length - 1]!
        return { client, view, socket }
    }

    beforeEach(() => {
        vi.useFakeTimers()
        FakeSocket.instances = []
        vi.stubGlobal('WebSocket', FakeSocket)
        useNotifications.setState({ items: [] })
        useSession.setState({ status: 'authenticated' })
    })

    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    it('opens one connection per mount', () => {
        mount()
        expect(FakeSocket.instances).toHaveLength(1)
        expect(FakeSocket.instances[0]?.url).toMatch(/\/ws$/)
    })

    it('shows a persistent notice while disconnected and hides it when back', () => {
        const { socket } = mount()
        socket().onmessage?.({ data: '{"type":"ready"}' })

        socket().onclose?.({ code: 1006 })
        expect(useNotifications.getState().items[0]).toMatchObject({
            id: 'network',
            persistent: true,
        })

        vi.advanceTimersByTime(1_000)
        socket().onmessage?.({ data: '{"type":"ready"}' })
        expect(useNotifications.getState().items).toHaveLength(0)
    })

    it('refetches data after a reconnect', () => {
        const { client, socket } = mount()
        const invalidate = vi.spyOn(client, 'invalidateQueries')
        socket().onmessage?.({ data: '{"type":"ready"}' })
        expect(invalidate).not.toHaveBeenCalled()

        socket().onclose?.({ code: 1006 })
        vi.advanceTimersByTime(1_000)
        socket().onmessage?.({ data: '{"type":"ready"}' })
        expect(invalidate).toHaveBeenCalledTimes(1)
    })

    it('signs the user out when the session was revoked', () => {
        const { client, socket } = mount()
        client.setQueryData(['me'], { username: 'me' })
        socket().onclose?.({ code: 4001 })

        expect(useSession.getState().status).toBe('guest')
        expect(client.getQueryData(['me'])).toBeUndefined()
    })

    it('rechecks the profile instead of signing out when it is missing', () => {
        const { client, socket } = mount()
        const invalidate = vi.spyOn(client, 'invalidateQueries')
        socket().onclose?.({ code: 4403 })

        expect(useSession.getState().status).toBe('authenticated')
        expect(invalidate).toHaveBeenCalledWith({ queryKey: ['me'] })
    })

    it('closes the connection on unmount', () => {
        const close = vi.spyOn(FakeSocket.prototype, 'close')
        const { view } = mount()
        view.unmount()
        expect(close).toHaveBeenCalled()
    })
})
