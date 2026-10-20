import type { ReactNode } from 'react'

import type { ConversationSummary } from '@/entities/conversation'
import { usePresence, UserAvatar, useUser } from '@/entities/user'
import { formatLastSeen, plural } from '@/shared/lib'
import { Status } from '@/shared/ui'

import styles from './ChatHeader.module.scss'

interface Props {
    // открытый чат; без него - новый личный чат с peerId, которого ещё нет на сервере
    chat?: ConversationSummary
    peerId?: string
    // клик по аватару и имени: профиль собеседника или сведения о группе
    onOpenInfo?: () => void
    // кнопки справа (меню чата)
    actions?: ReactNode
}

// шапка чата
export function ChatHeader({ chat, peerId, onOpenInfo, actions }: Props) {
    const isDirect = !chat || chat.conversation_type === 'direct'
    const peerUserId = chat ? chat.peer_id : peerId

    const peer = useUser(isDirect ? peerUserId : null)
    const presence = usePresence(isDirect ? peerUserId : null)

    const profile = peer.data
    const name = isDirect ? (profile?.display_name ?? '') : (chat?.name ?? 'Группа')
    const status = isDirect ? profile?.status : chat?.status
    const online = isDirect && (presence?.online ?? profile?.online ?? false)

    let subtitle = ''
    if (isDirect) {
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
                <UserAvatar
                    src={isDirect ? profile?.avatar_url : chat?.avatar_url}
                    shape={isDirect ? profile?.avatar_shape : chat?.avatar_shape}
                    online={online}
                />
                <div className={styles.texts}>
                    <h2 className={styles.name}>{name || ' '}</h2>
                    <span className={styles.subtitle}>{subtitle || ' '}</span>
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
