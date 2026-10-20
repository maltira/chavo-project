import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router'

import { applyConversationEvent, moveReadCursor, useGroupCard } from '@/entities/conversation'
import { applyMessageEvent } from '@/entities/message'
import { useSession } from '@/entities/session'
import { blockedKey, blockStatusKey, presenceKey, userKey } from '@/entities/user'
import type { Presence, Profile } from '@/entities/user'
import { connectSocket } from '@/shared/api'
import { chatPath } from '@/shared/config'
import type { ServerEvent } from '@/shared/api'
import { dismiss, notify } from '@/shared/lib'
import type { Notice } from '@/shared/lib'

import { applyGroupEvent, notifyGroupEvent } from './groupEvents'
import { notifyNewMessage } from './messageNotice'

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
            // блокировка прячет аватар, фигуру, онлайн и статус и запрещает писать:
            // перечитываем профили и статус блокировки обоих
            for (const id of [data.blocker_id, data.blocked_id]) {
                if (id) {
                    void queryClient.invalidateQueries({ queryKey: userKey(id) })
                    void queryClient.invalidateQueries({ queryKey: blockStatusKey(id) })
                }
            }
            // блокировали с другого моего устройства - чёрный список здесь тоже меняется
            void queryClient.invalidateQueries({ queryKey: blockedKey })
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
        default: {
            const myId = queryClient.getQueryData<Profile | null>(['me'])?.user_id
            applyMessageEvent(queryClient, event)
            applyConversationEvent(queryClient, event, myId)
            applyGroupEvent(event, queryClient, myId)
            break
        }
    }
}

// чем объяснить выход из аккаунта; reason - причина session.revoked или unauthorized (код 4401);
// null - объяснять не нужно (вышел сам на этом устройстве)
export function sessionEndedNotice(reason: string | undefined): Notice | null {
    switch (reason) {
        case 'logout':
            return null
        case 'password_changed':
            // то же id, что у уведомления формы смены пароля: на этом устройстве оно не задвоится
            return {
                id: 'password-changed',
                icon: 'lock',
                title: 'Пароль изменён',
                description: 'Все сеансы завершены, войдите с новым паролем',
            }
        case 'password_reset':
            return {
                id: 'session-ended',
                icon: 'lock',
                title: 'Пароль сброшен',
                description: 'Все сеансы завершены, войдите с новым паролем',
            }
        case 'unauthorized':
            // сервер не принял сеанс (истёк или не нашёлся): причину точно не знаем, не гадаем
            return {
                id: 'session-ended',
                icon: 'warn',
                title: 'Нужно войти снова',
                description: 'Сеанс больше не действует',
            }
        default:
            return {
                id: 'session-ended',
                icon: 'session',
                title: 'Сеанс завершён',
                description: 'Этот сеанс завершили с другого устройства',
            }
    }
}

// сколько ждать закрытия соединения после session.revoked, прежде чем выйти самим
const REVOKE_GRACE_MS = 2000

// единственное WebSocket-соединение вкладки; монтируется только когда есть сессия и профиль
export function RealtimeBridge() {
    const queryClient = useQueryClient()
    const signOut = useSession((state) => state.signOut)
    // клик по уведомлению открывает чат; navigate через ref, чтобы смена маршрута не переподключала сокет
    const navigate = useNavigate()
    const navigateRef = useRef(navigate)
    useEffect(() => {
        navigateRef.current = navigate
    }, [navigate])

    useEffect(() => {
        // перед закрытием соединения с кодом 4001 сервер присылает session.revoked с причиной
        let revokeReason: string | undefined
        // страховка: если после session.revoked соединение не закрылось, выходим сами
        let revokeTimer: ReturnType<typeof setTimeout> | undefined
        let ended = false
        const open = {
            chat: (id: string) => navigateRef.current(chatPath(id)),
            group: (id: string) => useGroupCard.getState().openChat(id),
        }

        function endSession(reason: string | undefined) {
            if (ended) {
                return
            }
            ended = true
            clearTimeout(revokeTimer)
            const notice = sessionEndedNotice(reason)
            if (notice) {
                notify(notice)
            }
            queryClient.clear()
            signOut()
        }

        const socket = connectSocket({
            onEvent: (event) => {
                if (event.type === 'session.revoked') {
                    revokeReason = (event.data as { reason?: string } | undefined)?.reason
                    clearTimeout(revokeTimer)
                    revokeTimer = setTimeout(() => {
                        endSession(revokeReason)
                        socket.close()
                    }, REVOKE_GRACE_MS)
                    return
                }
                applyEvent(event, queryClient)
                const myId = queryClient.getQueryData<Profile | null>(['me'])?.user_id
                void notifyNewMessage(event, queryClient, myId, open)
                void notifyGroupEvent(event, queryClient, myId, open)
            },
            onOpen: ({ reconnected }) => {
                dismiss('network')
                // новое соединение - причина прошлого закрытия к нему не относится
                revokeReason = undefined
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
                // сессия завершена: на другом устройстве, при смене пароля или сервер её не принял
                endSession(reason === 'revoked' ? revokeReason : 'unauthorized')
            },
        })
        return () => {
            clearTimeout(revokeTimer)
            socket.close()
        }
    }, [queryClient, signOut])

    return null
}
