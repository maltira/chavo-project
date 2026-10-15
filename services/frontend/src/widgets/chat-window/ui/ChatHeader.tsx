import type { ConversationSummary } from '@/entities/conversation'
import { useMembers } from '@/entities/conversation'
import { usePresence, UserAvatar, useUser } from '@/entities/user'
import { formatLastSeen, plural } from '@/shared/lib'
import { Status } from '@/shared/ui'

import styles from './ChatHeader.module.scss'

interface Props {
    chat: ConversationSummary
}

// шапка чата
export function ChatHeader({ chat }: Props) {
    const isDirect = chat.conversation_type === 'direct'

    const peer = useUser(isDirect ? chat.peer_id : null)
    const presence = usePresence(isDirect ? chat.peer_id : null)
    const members = useMembers(chat.id)

    const profile = peer.data
    const name = isDirect ? (profile?.display_name ?? '') : (chat.name ?? 'Группа')
    const status = isDirect ? profile?.status : chat.status
    const online = isDirect && (presence?.online ?? profile?.online ?? false)

    let subtitle = ''
    if (isDirect) {
        subtitle = online
            ? 'в сети'
            : formatLastSeen(presence?.lastSeenAt ?? profile?.last_seen_at ?? null)
    } else if (members.data) {
        // сервер отдаёт до 100 участников за раз
        const count = members.data.length
        subtitle = `${count >= 100 ? '100+' : count} ${plural(count, ['участник', 'участника', 'участников'])}`
    }

    return (
        <header className={styles.root}>
            <UserAvatar
                src={isDirect ? profile?.avatar_url : chat.avatar_url}
                shape={isDirect ? profile?.avatar_shape : chat.avatar_shape}
                online={online}
            />
            <div className={styles.texts}>
                <h2 className={styles.name}>{name || ' '}</h2>
                <span className={styles.subtitle}>{subtitle || ' '}</span>
            </div>
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
        </header>
    )
}
