import { create } from 'zustand'

import { refreshAccessToken, setAccessToken, setSessionLostHandler } from '@/shared/api'

// unknown - ещё не проверили refresh-cookie при старте
export type SessionStatus = 'unknown' | 'authenticated' | 'guest'

// почему сессию пока не удаётся проверить: offline - нет интернета, server - сервер не отвечает
export type Unreachable = 'offline' | 'server'

interface SessionState {
    status: SessionStatus
    // пока status === 'unknown': null - проверка идёт, иначе причина, по которой ждём повтора
    unreachable: Unreachable | null
    signIn: (accessToken: string) => void
    signOut: () => void
}

export const useSession = create<SessionState>()((set) => ({
    status: 'unknown',
    unreachable: null,
    signIn: (accessToken) => {
        setAccessToken(accessToken)
        set({ status: 'authenticated', unreachable: null })
    },
    signOut: () => {
        setAccessToken(null)
        set({ status: 'guest', unreachable: null })
    },
}))

// сессия потеряна посреди работы (refresh не удался) - возвращаем гостя на вход
setSessionLostHandler(() => useSession.getState().signOut())

// паузы между повторами при старте без связи: 2 с, затем вдвое дольше, но не больше 30 с
const RETRY_FIRST_MS = 2_000
const RETRY_MAX_MS = 30_000

let retryTimer: ReturnType<typeof setTimeout> | undefined
let retryDelay = RETRY_FIRST_MS

function onOnline() {
    // сеть вернулась - не ждём таймера
    clearTimeout(retryTimer)
    void bootstrapSession()
}

// при старте приложения: пробуем восстановить сессию по refresh-cookie.
// нет ответа (сеть или сервер) - не выходим: остаёмся на заставке с причиной и повторяем сами,
// иначе человек без интернета увидел бы форму входа, будто его разлогинило
export async function bootstrapSession() {
    const result = await refreshAccessToken()
    if (result.status !== 'unavailable') {
        clearTimeout(retryTimer)
        window.removeEventListener('online', onOnline)
        retryDelay = RETRY_FIRST_MS
        if (result.status === 'ok') {
            useSession.getState().signIn(result.token)
        } else {
            useSession.getState().signOut()
        }
        return
    }

    useSession.setState({ unreachable: navigator.onLine ? 'server' : 'offline' })
    window.addEventListener('online', onOnline)
    clearTimeout(retryTimer)
    retryTimer = setTimeout(() => void bootstrapSession(), retryDelay)
    retryDelay = Math.min(retryDelay * 2, RETRY_MAX_MS)
}
