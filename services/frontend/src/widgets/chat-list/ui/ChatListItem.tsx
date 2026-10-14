import { NavLink } from 'react-router'

import type { ConversationSummary } from '@/entities/conversation'
import { usePresence, UserAvatar, useUser } from '@/entities/user'
import { chatPath } from '@/shared/config'
import { cn, formatChatTime } from '@/shared/lib'
import { Icon, Tag } from '@/shared/ui'

import styles from './ChatListItem.module.scss'

interface Props {
    chat: ConversationSummary
    myId: string | undefined
    compact: boolean
}

// больше двух цифр бейдж не показывает
const unreadLabel = (count: number) => (count > 99 ? '99+' : String(count))

function preview(chat: ConversationSummary, myId: string | undefined) {
    const message = chat.last_message
    if (!message) {
        return chat.conversation_type === 'group' ? 'Группа создана' : 'Нет сообщений'
    }
    const text = message.is_deleted ? 'Сообщение удалено' : (message.content ?? '')
    return message.sender_id === myId ? `Вы: ${text}` : text
}

// строка чата: аватар, имя, последнее сообщение, время, галочки и непрочитанные
export function ChatListItem({ chat, myId, compact }: Props) {
    const isDirect = chat.conversation_type === 'direct'

    const peer = useUser(isDirect ? chat.peer_id : null)
    const presence = usePresence(isDirect ? chat.peer_id : null)

    const name = isDirect ? (peer.data?.display_name ?? '') : (chat.name ?? 'Группа')
    const avatarUrl = isDirect ? peer.data?.avatar_url : chat.avatar_url
    const online = isDirect && (presence?.online ?? peer.data?.online ?? false)

    const mine = !!chat.last_message && chat.last_message.sender_id === myId
    const unread = chat.unread_count > 0
    const time = formatChatTime(chat.last_message?.created_at ?? chat.created_at)

    return (
        <NavLink
            to={chatPath(chat.id)}
            className={({ isActive }) =>
                cn(styles.item, isActive && styles.active, compact && styles.compact)
            }
            aria-label={compact ? name : undefined}
            title={compact ? name : undefined}
        >
            <span className={styles.avatar}>
                <UserAvatar src={avatarUrl} size={44} online={online} />
                {compact && unread && (
                    <span className={styles.badge}>{unreadLabel(chat.unread_count)}</span>
                )}
            </span>

            {!compact && (
                <>
                    <span className={cn(styles.texts, unread && styles.unread)}>
                        <span className={styles.name}>{name || ' '}</span>
                        <span className={styles.preview}>{preview(chat, myId)}</span>
                    </span>

                    <span className={styles.meta}>
                        <span className={styles.time}>
                            {mine && (
                                <Icon
                                    name={chat.last_message_read ? 'check-double' : 'check'}
                                    size={14}
                                    color="var(--color-accent)"
                                />
                            )}
                            {time}
                        </span>
                        {unread && <Tag>{unreadLabel(chat.unread_count)}</Tag>}
                    </span>
                </>
            )}
        </NavLink>
    )
}
