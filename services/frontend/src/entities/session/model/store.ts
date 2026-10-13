import { create } from 'zustand'

import { refreshAccessToken, setAccessToken, setSessionLostHandler } from '@/shared/api'

// unknown - ещё не проверили refresh-cookie при старте
export type SessionStatus = 'unknown' | 'authenticated' | 'guest'

interface SessionState {
    status: SessionStatus
    signIn: (accessToken: string) => void
    signOut: () => void
}

export const useSession = create<SessionState>()((set) => ({
    status: 'unknown',
    signIn: (accessToken) => {
        setAccessToken(accessToken)
        set({ status: 'authenticated' })
    },
    signOut: () => {
        setAccessToken(null)
        set({ status: 'guest' })
    },
}))

// сессия потеряна посреди работы (refresh не удался) - возвращаем гостя на вход
setSessionLostHandler(() => useSession.getState().signOut())

// при старте приложения: пробуем восстановить сессию по refresh-cookie
// если ответа нет (сеть), показываем вход, но cookie не трогаем: после перезагрузки сессия вернётся
export async function bootstrapSession() {
    const result = await refreshAccessToken()
    if (result.status === 'ok') {
        useSession.getState().signIn(result.token)
    } else {
        useSession.getState().signOut()
    }
}
