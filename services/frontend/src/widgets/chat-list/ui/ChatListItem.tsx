import { useState } from 'react'
import { NavLink } from 'react-router'

import { isMuted, SAVED_NAME, SavedAvatar } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { isSystem, useSystemText } from '@/entities/message'
import { usePresence, UserAvatar, useUser } from '@/entities/user'
import { ChatActionsMenu } from '@/features/chat-actions'
import { chatPath } from '@/shared/config'
import { cn, formatChatTime } from '@/shared/lib'
import { Icon, Tag } from '@/shared/ui'
import type { MenuPosition } from '@/shared/ui'

import styles from './ChatListItem.module.scss'

interface Props {
    chat: ConversationSummary
    myId: string | undefined
    compact: boolean
}

// больше двух цифр бейдж не показывает
const unreadLabel = (count: number) => (count > 99 ? '99+' : String(count))

// systemText - текст системного последнего сообщения (вступил, покинул): без «Вы:»
function preview(chat: ConversationSummary, myId: string | undefined, systemText: string) {
    const message = chat.last_message
    if (!message) {
        return chat.conversation_type === 'group' ? 'Группа создана' : 'Нет сообщений'
    }
    if (isSystem(message)) {
        return systemText
    }
    const text = message.is_deleted ? 'Сообщение удалено' : (message.content ?? '')
    // в «Избранном» все сообщения свои - без «Вы:»
    return message.sender_id === myId && !chat.is_saved ? `Вы: ${text}` : text
}

// строка чата: аватар, имя, последнее сообщение, время, галочки и непрочитанные
export function ChatListItem({ chat, myId, compact }: Props) {
    const isDirect = chat.conversation_type === 'direct'
    const saved = chat.is_saved

    const peer = useUser(isDirect ? chat.peer_id : null)
    const presence = usePresence(isDirect ? chat.peer_id : null)

    const name = saved
        ? SAVED_NAME
        : isDirect
          ? (peer.data?.display_name ?? '')
          : (chat.name ?? 'Группа')
    const avatarUrl = isDirect ? peer.data?.avatar_url : chat.avatar_url
    const online = isDirect && (presence?.online ?? peer.data?.online ?? false)

    const systemText = useSystemText(chat.last_message)
    // галочки - только у моих обычных сообщений; в «Избранном» читать некому
    const mine =
        !saved &&
        !!chat.last_message &&
        chat.last_message.sender_id === myId &&
        !isSystem(chat.last_message)
    const unread = chat.unread_count > 0
    const muted = isMuted(chat)
    const time = formatChatTime(chat.last_message?.created_at ?? chat.created_at)

    // меню чата по правой кнопке - у курсора
    const [menuAt, setMenuAt] = useState<MenuPosition | null>(null)

    return (
        <>
            <NavLink
                to={chatPath(chat.id)}
                onContextMenu={(event) => {
                    event.preventDefault()
                    setMenuAt({ x: event.clientX, y: event.clientY })
                }}
                className={({ isActive }) =>
                    cn(styles.item, isActive && styles.active, compact && styles.compact)
                }
                aria-label={compact ? name : undefined}
                title={compact ? name : undefined}
            >
                <span className={styles.avatar}>
                    {saved ? (
                        <SavedAvatar size={44} />
                    ) : (
                        <UserAvatar src={avatarUrl} size={44} online={online} />
                    )}
                    {compact && unread && (
                        <span className={cn(styles.badge, muted && styles.mutedBadge)}>
                            {unreadLabel(chat.unread_count)}
                        </span>
                    )}
                </span>

                {!compact && (
                    <>
                        <span className={cn(styles.texts, unread && styles.unread)}>
                            <span className={styles.name}>{name || ' '}</span>
                            <span className={styles.preview}>
                                {preview(chat, myId, systemText)}
                            </span>
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
                            <span className={styles.flags}>
                                {muted && (
                                    <span role="img" aria-label="Без звука" className={styles.flag}>
                                        <Icon name="notif-off" size={16} color="currentColor" />
                                    </span>
                                )}
                                {chat.pinned_at && !unread && (
                                    <span role="img" aria-label="Закреплён" className={styles.flag}>
                                        <Icon name="pinned" size={16} color="currentColor" />
                                    </span>
                                )}
                                {unread && (
                                    <Tag variant={muted ? 'muted' : 'default'}>
                                        {unreadLabel(chat.unread_count)}
                                    </Tag>
                                )}
                            </span>
                        </span>
                    </>
                )}
            </NavLink>
            {/* меню вне ссылки: события из него не должны переходить по ней */}
            <ChatActionsMenu chat={chat} position={menuAt} onClose={() => setMenuAt(null)} />
        </>
    )
}
