import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'

import { applyConversationEvent, moveReadCursor } from '@/entities/conversation'
import { applyMessageEvent } from '@/entities/message'
import { useSession } from '@/entities/session'
import { presenceKey, userKey } from '@/entities/user'
import type { Presence, Profile } from '@/entities/user'
import { connectSocket } from '@/shared/api'
import type { ServerEvent } from '@/shared/api'
import { dismiss, notify } from '@/shared/lib'

// события сервера обновляют кеш запросов, а не состояние компонентов: экраны уже читают кеш
export function applyEvent(event: ServerEvent, queryClient: QueryClient) {
    const data = (event.data ?? {}) as {
        user_id?: string
        last_seen_at?: string | null
        blocker_id?: string
        blocked_id?: string
    }

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
        case 'user.blocked':
        case 'user.unblocked':
            // блокировка прячет аватар, фигуру, онлайн и статус: перечитываем оба профиля
            for (const id of [data.blocker_id, data.blocked_id]) {
                if (id) {
                    void queryClient.invalidateQueries({ queryKey: userKey(id) })
                }
            }
            break
        case 'message.read': {
            // курсор участника - для галочек в открытом чате
            const read = event.data as {
                conversation_id?: string
                reader_id?: string
                message_id?: string
            }
            if (read.conversation_id && read.reader_id && read.message_id) {
                moveReadCursor(queryClient, read.conversation_id, read.reader_id, read.message_id)
            }
            applyConversationEvent(
                queryClient,
                event,
                queryClient.getQueryData<Profile | null>(['me'])?.user_id,
            )
            break
        }
        default:
            applyMessageEvent(queryClient, event)
            applyConversationEvent(
                queryClient,
                event,
                queryClient.getQueryData<Profile | null>(['me'])?.user_id,
            )
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
