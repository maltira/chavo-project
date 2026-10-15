import { useCallback, useState } from 'react'

import { useConversation, useMembers } from '@/entities/conversation'
import { useMessages } from '@/entities/message'
import type { Message } from '@/entities/message'
import { useMe, useUser } from '@/entities/user'
import { MessageComposer } from '@/features/compose-message'
import type { ReplyTarget } from '@/features/compose-message'
import { useDeleteMessage } from '@/features/delete-message'
import { useMarkRead } from '@/features/mark-read'
import { Button, Icon, Loader } from '@/shared/ui'

import { firstUnreadId, readBoundary } from '../lib/readState'
import { ChatHeader } from './ChatHeader'
import styles from './ChatWindow.module.scss'
import { MessageList } from './MessageList'

interface Props {
    conversationId: string
}

// предел догрузки при переходе по цитате: дальше сообщение считается недоступным
const MAX_PAGES_TO_FIND = 20

// окно открытого чата
export function ChatWindow({ conversationId }: Props) {
    const me = useMe()
    const myId = me.data?.user_id
    const chat = useConversation(conversationId)
    const history = useMessages(conversationId)
    const members = useMembers(conversationId)
    const remove = useDeleteMessage()

    const [reply, setReply] = useState<Message | null>(null)
    const [editing, setEditing] = useState<Message | null>(null)
    const replyAuthor = useUser(reply && reply.sender_id !== myId ? reply.sender_id : null)

    const messages = history.data ?? []

    // число непрочитанных запоминается при открытии: прочтение сразу обнулит его в списке
    const [unreadAtOpen] = useState(() => chat.data?.unread_count ?? 0)
    // разделитель ставится один раз, когда история загрузилась, и потом не двигается
    const [divider, setDivider] = useState<{ id: string | null } | null>(null)
    if (divider === null && messages.length > 0) {
        setDivider({ id: firstUnreadId(messages, unreadAtOpen, myId) })
    }
    // самое свежее сообщение, которое побывало на экране; до него курсор прочтения
    const [seenId, setSeenId] = useState<string>()
    useMarkRead(conversationId, seenId)

    const { fetchNextPage } = history
    const loadOlder = useCallback(
        () => void fetchNextPage({ cancelRefetch: false }),
        [fetchNextPage],
    )

    // переход по цитате к старому сообщению: догружаем историю страницами, пока не найдём
    const findMessage = useCallback(
        async (id: string) => {
            for (let page = 0; page < MAX_PAGES_TO_FIND; page++) {
                const result = await fetchNextPage({ cancelRefetch: false })
                const pages = result.data?.pages ?? []
                if (pages.some((p) => p.items.some((m) => m.id === id))) {
                    return true
                }
                if (!result.hasNextPage) {
                    return false
                }
            }
            return false
        },
        [fetchNextPage],
    )

    if (chat.isError) {
        return (
            <div className={styles.state}>
                <Icon name="shape-warn" size={67} color="var(--color-text)" />
                <p className={styles.stateText}>Чат недоступен</p>
            </div>
        )
    }

    if (!chat.data) {
        return (
            <div className={styles.state}>
                <Loader size={48} />
            </div>
        )
    }

    const replyTarget: ReplyTarget | null = reply
        ? {
              id: reply.id,
              author: reply.sender_id === myId ? 'Вы' : (replyAuthor.data?.display_name ?? ''),
              text: reply.content ?? '',
          }
        : null

    return (
        <div className={styles.root}>
            <ChatHeader chat={chat.data} />

            {history.isPending ? (
                <div className={styles.state}>
                    <Loader size={48} />
                </div>
            ) : history.isError && messages.length === 0 ? (
                <div className={styles.state}>
                    <p className={styles.stateText}>Не удалось загрузить сообщения</p>
                    <Button
                        variant="secondary"
                        className={styles.retry}
                        onClick={() => void history.refetch()}
                    >
                        Повторить
                    </Button>
                </div>
            ) : messages.length === 0 ? (
                <div className={styles.state}>
                    <Icon name="shape-message" size={67} color="var(--color-text)" />
                    <p className={styles.stateText}>Сообщений пока нет</p>
                </div>
            ) : (
                <MessageList
                    chat={chat.data}
                    messages={messages}
                    myId={myId}
                    readBoundary={readBoundary(messages, members.data, myId)}
                    hasOlder={history.hasNextPage}
                    loadingOlder={history.isFetchingNextPage}
                    loadOlder={loadOlder}
                    firstUnreadId={divider?.id ?? null}
                    onSeen={setSeenId}
                    findMessage={findMessage}
                    onReply={(message) => {
                        setEditing(null)
                        setReply(message)
                    }}
                    onEdit={(message) => {
                        setReply(null)
                        setEditing(message)
                    }}
                    onDelete={(message) => remove.mutate(message)}
                />
            )}

            <div className={styles.composer}>
                <MessageComposer
                    key={editing?.id ?? 'new'}
                    conversationId={conversationId}
                    myId={myId}
                    replyTo={replyTarget}
                    editing={editing}
                    onCancel={() => {
                        setReply(null)
                        setEditing(null)
                    }}
                />
            </div>
        </div>
    )
}
