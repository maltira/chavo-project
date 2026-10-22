import { useQueryClient } from '@tanstack/react-query'
import { Navigate, useNavigate } from 'react-router'

import { conversationsKey, findDirectWith, findSaved } from '@/entities/conversation'
import { useMe, useProfileCard } from '@/entities/user'
import { MessageComposer } from '@/features/compose-message'
import { chatPath } from '@/shared/config'

import { BlockedNotice, useCanWrite } from './BlockedNotice'
import { ChatHeader } from './ChatHeader'
import styles from './ChatWindow.module.scss'

interface Props {
    userId: string
}

// личный чат, которого ещё нет: шапка собеседника и пустая история; с самим собой - «Избранное»;
// первое сообщение создаёт чат на сервере, и окно переходит на его адрес
export function NewChatWindow({ userId }: Props) {
    const me = useMe()
    const queryClient = useQueryClient()
    const navigate = useNavigate()
    const openProfile = useProfileCard((state) => state.open)
    const saved = userId === me.data?.user_id
    // себе писать можно всегда: блокировок с самим собой нет
    const canWrite = useCanWrite(saved ? null : userId)

    // чат уже есть (появился в списке или открыли старую ссылку) - сразу в него
    const existing = saved ? findSaved(queryClient) : findDirectWith(queryClient, userId)
    if (existing) {
        return <Navigate to={chatPath(existing.id)} replace />
    }

    return (
        <div className={styles.root}>
            {saved ? (
                <ChatHeader saved />
            ) : (
                <ChatHeader peerId={userId} onOpenInfo={() => openProfile(userId)} />
            )}

            <div className={styles.state}>
                <p className={styles.stateText}>
                    {saved
                        ? 'Сохраняйте сюда заметки, ссылки и пересланные сообщения'
                        : 'Напишите первое сообщение'}
                </p>
            </div>

            <div className={styles.composer}>
                {canWrite ? (
                    <MessageComposer
                        recipientId={userId}
                        myId={me.data?.user_id}
                        onCancel={() => {}}
                        onSent={(message) => {
                            // новый чат появится в списке после перечитывания
                            void queryClient.invalidateQueries({ queryKey: conversationsKey })
                            navigate(chatPath(message.conversation_id), { replace: true })
                        }}
                    />
                ) : (
                    <BlockedNotice peerId={userId} />
                )}
            </div>
        </div>
    )
}
