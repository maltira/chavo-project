import type { ReactNode } from 'react'

import {
    SAVED_NAME,
    SavedAvatar,
    TypingIndicator,
    useTypingUsers,
} from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { usePresence, UserAvatar, useUser } from '@/entities/user'
import { formatLastSeen, plural } from '@/shared/lib'
import { Status } from '@/shared/ui'

import styles from './ChatHeader.module.scss'

interface Props {
    // открытый чат; без него - новый личный чат с peerId, которого ещё нет на сервере
    chat?: ConversationSummary
    peerId?: string
    // черновик «Избранного» (чата с собой ещё нет)
    saved?: boolean
    // клик по аватару и имени: профиль собеседника или сведения о группе
    onOpenInfo?: () => void
    // кнопки справа (меню чата)
    actions?: ReactNode
}

// шапка чата
export function ChatHeader({ chat, peerId, saved: savedDraft, onOpenInfo, actions }: Props) {
    const saved = chat ? chat.is_saved : !!savedDraft
    const isDirect = !chat || chat.conversation_type === 'direct'
    // у «Избранного» собеседника нет: ни профиля, ни «в сети»
    const peerUserId = saved ? null : chat ? chat.peer_id : peerId

    const peer = useUser(isDirect ? peerUserId : null)
    const presence = usePresence(isDirect ? peerUserId : null)
    const typingIds = useTypingUsers(chat?.id)
    const firstTyper = useUser(!isDirect && typingIds.length > 0 ? typingIds[0]! : null)

    const profile = peer.data
    const isDeleted = isDirect && !!profile?.is_deleted
    const name = isDirect
        ? isDeleted
            ? 'Удалённый аккаунт'
            : (profile?.display_name ?? '')
        : (chat?.name ?? 'Группа')
    const status = isDeleted ? undefined : isDirect ? profile?.status : chat?.status
    const online = !isDeleted && isDirect && (presence?.online ?? profile?.online ?? false)

    const showTyping = !saved && !isDeleted && typingIds.length > 0
    let typingLabel = ''
    let subtitle = ''
    if (showTyping) {
        if (isDirect) {
            typingLabel = 'печатает'
        } else {
            const firstName = firstTyper.data?.display_name ?? 'Кто-то'
            typingLabel =
                typingIds.length === 1
                    ? `${firstName} печатает`
                    : `${firstName} и ещё ${typingIds.length - 1} печатают`
        }
    } else if (saved) {
        subtitle = 'Сообщения для себя'
    } else if (isDeleted) {
        subtitle = 'был(а) давно'
    } else if (isDirect) {
        subtitle = online
            ? 'в сети'
            : formatLastSeen(presence?.lastSeenAt ?? profile?.last_seen_at ?? null)
    } else if (chat) {
        const count = chat.members_count
        subtitle = `${count} ${plural(count, ['участник', 'участника', 'участников'])}`
    }

    return (
        <header className={styles.root}>
            <button
                type="button"
                className={styles.who}
                disabled={!onOpenInfo}
                aria-label={isDirect ? 'Профиль собеседника' : 'Сведения о группе'}
                onClick={onOpenInfo}
            >
                {saved ? (
                    <SavedAvatar />
                ) : (
                    <UserAvatar
                        src={isDeleted ? undefined : isDirect ? profile?.avatar_url : chat?.avatar_url}
                        shape={
                            isDeleted
                                ? undefined
                                : isDirect
                                  ? profile?.avatar_shape
                                  : chat?.avatar_shape
                        }
                        online={online}
                    />
                )}
                <div className={styles.texts}>
                    <h2 className={styles.name}>{saved ? SAVED_NAME : name || ' '}</h2>
                    <span className={styles.subtitle}>
                        {showTyping ? <TypingIndicator label={typingLabel} /> : subtitle || ' '}
                    </span>
                </div>
            </button>
            {status && (
                <Status
                    icon={status.icon}
                    text={status.text}
                    background={status.background}
                    foreground={status.foreground}
                    expiresAt={status.expires_at}
                    className={styles.status}
                />
            )}
            {actions}
        </header>
    )
}
