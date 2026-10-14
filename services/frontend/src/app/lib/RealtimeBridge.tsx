import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'

import { useSession } from '@/entities/session'
import { connectSocket } from '@/shared/api'
import type { ServerEvent } from '@/shared/api'
import { dismiss, notify } from '@/shared/lib'

export const presenceKey = (userId: string) => ['presence', userId] as const

export interface Presence {
    online: boolean
    lastSeenAt: string | null
}

// события сервера обновляют кеш запросов, а не состояние компонентов: экраны уже читают кеш
export function applyEvent(event: ServerEvent, queryClient: QueryClient) {
    const data = (event.data ?? {}) as { user_id?: string; last_seen_at?: string | null }

    switch (event.type) {
        case 'user.online':
            if (data.user_id) {
                queryClient.setQueryData<Presence>(presenceKey(data.user_id), {
                    online: true,
                    lastSeenAt: null,
                })
            }
            break
        case 'user.offline':
            if (data.user_id) {
                queryClient.setQueryData<Presence>(presenceKey(data.user_id), {
                    online: false,
                    lastSeenAt: data.last_seen_at ?? null,
                })
            }
            break
        default:
            // остальные события (сообщения, чаты) подключаются вместе с соответствующими экранами
            break
    }
}

// единственное WebSocket-соединение вкладки; монтируется только когда есть сессия и профиль
export function RealtimeBridge() {
    const queryClient = useQueryClient()
    const signOut = useSession((state) => state.signOut)

    useEffect(() => {
        const socket = connectSocket({
            onEvent: (event) => applyEvent(event, queryClient), // обработка событий
            onOpen: ({ reconnected }) => {
                dismiss('network')
                // пока связи не было, события не доставлялись: данные перечитываем
                if (reconnected) {
                    void queryClient.invalidateQueries()
                }
            },
            onDisconnect: () =>
                notify({
                    id: 'network',
                    icon: 'warn',
                    title: 'Нет соединения',
                    description: 'Пробуем подключиться снова',
                    persistent: true,
                }),
            onFatal: (reason) => {
                dismiss('network')
                if (reason === 'profile_required') {
                    void queryClient.invalidateQueries({ queryKey: ['me'] })
                    return
                }
                // сессия завершена: на другом устройстве, при смене пароля или истекла
                queryClient.clear()
                signOut()
            },
        })
        return () => socket.close()
    }, [queryClient, signOut])

    return null
}
