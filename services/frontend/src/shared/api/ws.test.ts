import { connectSocket } from './ws'
import type { FatalReason, ServerEvent } from './ws'

class FakeSocket {
    static OPEN = 1
    static instances: FakeSocket[] = []
    readyState = 1
    sent: string[] = []
    onmessage: ((event: { data: string }) => void) | null = null
    onclose: ((event: { code: number }) => void) | null = null
    closed = false

    constructor(public url: string) {
        FakeSocket.instances.push(this)
    }
    send(data: string) {
        this.sent.push(data)
    }
    close() {
        this.closed = true
    }
    // помощники для тестов
    receive(frame: object) {
        this.onmessage?.({ data: JSON.stringify(frame) })
    }
    drop(code: number) {
        this.onclose?.({ code })
    }
}

function setup() {
    const events: ServerEvent[] = []
    const calls = { open: [] as boolean[], disconnect: 0, fatal: [] as FatalReason[] }
    const socket = connectSocket({
        url: 'ws://test/ws',
        random: () => 1,
        createSocket: (url) => new FakeSocket(url) as unknown as WebSocket,
        onEvent: (event) => events.push(event),
        onOpen: ({ reconnected }) => calls.open.push(reconnected),
        onDisconnect: () => (calls.disconnect += 1),
        onFatal: (reason) => calls.fatal.push(reason),
    })
    const last = () => FakeSocket.instances[FakeSocket.instances.length - 1]!
    return { socket, events, calls, last }
}

describe('connectSocket', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        FakeSocket.instances = []
        vi.stubGlobal('WebSocket', FakeSocket)
    })

    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    it('connects to the given url and reports ready', () => {
        const { last, calls } = setup()
        expect(last().url).toBe('ws://test/ws')
        last().receive({ type: 'ready', connection_id: 'sid:abc' })
        expect(calls.open).toEqual([false])
    })

    it('passes events on and drops repeated event ids', () => {
        const { last, events } = setup()
        last().receive({ type: 'ready' })
        last().receive({ type: 'message.created', event_id: 'e1', data: { a: 1 } })
        last().receive({ type: 'message.created', event_id: 'e1', data: { a: 1 } })
        last().receive({ type: 'message.created', event_id: 'e2' })

        expect(events.map((event) => event.event_id)).toEqual(['e1', 'e2'])
    })

    it('ignores broken frames, pong and error frames', () => {
        const { last, events } = setup()
        last().onmessage?.({ data: 'not json' })
        last().receive({ type: 'pong' })
        last().receive({ type: 'error', reason: 'UNKNOWN_TYPE' })
        last().receive({ nope: true })
        expect(events).toEqual([])
    })

    it('reconnects with a growing pause and marks the next ready as a reconnect', () => {
        const { last, calls } = setup()
        last().receive({ type: 'ready' })

        last().drop(1006)
        expect(calls.disconnect).toBe(1)
        expect(FakeSocket.instances).toHaveLength(1)

        vi.advanceTimersByTime(1_000)
        expect(FakeSocket.instances).toHaveLength(2)

        // связи так и нет: пауза вырастает до 2 секунд
        last().drop(1006)
        vi.advanceTimersByTime(1_999)
        expect(FakeSocket.instances).toHaveLength(2)
        vi.advanceTimersByTime(1)
        expect(FakeSocket.instances).toHaveLength(3)

        last().receive({ type: 'ready' })
        expect(calls.open).toEqual([false, true])
    })

    it.each([
        [4401, 'unauthorized'],
        [4403, 'profile_required'],
        [4001, 'revoked'],
    ] as const)('stops on close code %i', (code, reason) => {
        const { last, calls } = setup()
        last().drop(code)

        expect(calls.fatal).toEqual([reason])
        vi.advanceTimersByTime(60_000)
        expect(FakeSocket.instances).toHaveLength(1)
        expect(calls.disconnect).toBe(0)
    })

    it('does not reconnect after an intentional close', () => {
        const { socket, last, calls } = setup()
        socket.close()
        expect(last().closed).toBe(true)
        last().drop(1000)

        vi.advanceTimersByTime(60_000)
        expect(FakeSocket.instances).toHaveLength(1)
        expect(calls.disconnect).toBe(0)
    })

    it('sends JSON while the socket is open', () => {
        const { socket, last } = setup()
        socket.send({ type: 'ping' })
        expect(last().sent).toEqual(['{"type":"ping"}'])
    })
})
