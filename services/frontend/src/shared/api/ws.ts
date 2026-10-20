// клиент WebSocket Gateway: подключение по refresh-cookie, переподключение, дедупликация событий

export interface ServerEvent {
    type: string
    event_id?: string
    occurred_at?: string
    data?: unknown
    reason?: string
    error?: string
}

// причины, после которых переподключаться бессмысленно: решает приложение
export type FatalReason = 'unauthorized' | 'profile_required' | 'revoked'

const FATAL_CODES: Record<number, FatalReason> = {
    4001: 'revoked',
    4401: 'unauthorized',
    4403: 'profile_required',
}

const MIN_DELAY = 1_000
const MAX_DELAY = 30_000
// сервер доставляет событие хотя бы раз: повторы отсекаем по event_id
const SEEN_LIMIT = 500

interface Options {
    onEvent: (event: ServerEvent) => void
    // сервер прислал ready; reconnected - это не первое подключение: данные могли устареть
    onOpen?: (info: { reconnected: boolean }) => void
    // связь потеряна, клиент будет пробовать снова
    onDisconnect?: () => void
    onFatal?: (reason: FatalReason) => void
    url?: string
    // для тестов
    createSocket?: (url: string) => WebSocket
    random?: () => number
}

export interface Socket {
    send: (message: object) => void
    close: () => void
}

function defaultUrl() {
    const scheme = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${scheme}://${location.host}/ws`
}

export function connectSocket({
    onEvent,
    onOpen,
    onDisconnect,
    onFatal,
    url = defaultUrl(),
    createSocket = (target) => new WebSocket(target),
    random = Math.random,
}: Options): Socket {
    let socket: WebSocket | null = null
    let stopped = false
    let attempt = 0
    let everReady = false
    let timer: ReturnType<typeof setTimeout> | undefined
    const seen = new Set<string>()
    const order: string[] = []

    function remember(id: string) {
        if (seen.has(id)) {
            return false
        }
        seen.add(id)
        order.push(id)
        if (order.length > SEEN_LIMIT) {
            seen.delete(order.shift()!)
        }
        return true
    }

    function schedule() {
        // экспоненциальная пауза с разбросом, чтобы клиенты не переподключались одновременно
        const base = Math.min(MAX_DELAY, MIN_DELAY * 2 ** attempt)
        attempt += 1
        timer = setTimeout(connect, base * (0.5 + random() * 0.5))
    }

    function connect() {
        const current = createSocket(url)
        socket = current

        current.onmessage = (message: MessageEvent) => {
            let frame: ServerEvent
            try {
                frame = JSON.parse(String(message.data)) as ServerEvent
            } catch {
                return
            }
            if (!frame || typeof frame.type !== 'string') {
                return
            }
            if (frame.type === 'ready') {
                const reconnected = everReady
                everReady = true
                attempt = 0
                onOpen?.({ reconnected })
                return
            }
            if (frame.type === 'pong' || frame.type === 'error') {
                return
            }
            if (frame.event_id && !remember(frame.event_id)) {
                return
            }
            onEvent(frame)
        }

        current.onclose = (event: CloseEvent) => {
            if (stopped || socket !== current) {
                return
            }
            const fatal = FATAL_CODES[event.code]
            if (fatal) {
                stopped = true
                onFatal?.(fatal)
                return
            }
            onDisconnect?.()
            schedule()
        }
    }

    connect()

    return {
        send(message) {
            if (socket?.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify(message))
            }
        },
        close() {
            stopped = true
            clearTimeout(timer)
            socket?.close()
        },
    }
}
