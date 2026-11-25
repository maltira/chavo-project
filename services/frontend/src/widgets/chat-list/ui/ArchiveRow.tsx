import type { ConversationSummary } from '@/entities/conversation'
import { cn, plural } from '@/shared/lib'
import { Icon, Tag } from '@/shared/ui'

import itemStyles from './ChatListItem.module.scss'
import styles from './ArchiveRow.module.scss'

interface RowProps {
    chats: ConversationSummary[]
    // загружены не все архивные чаты
    more: boolean
    compact: boolean
    onOpen: () => void
}

const unreadLabel = (count: number) => (count > 99 ? '99+' : String(count))

// вход в архив над списком: сколько там чатов и непрочитанных (серым - архив не торопит)
export function ArchiveRow({ chats, more, compact, onOpen }: RowProps) {
    const unread = chats.reduce((sum, c) => sum + c.unread_count, 0)
    const count = `${chats.length}${more ? '+' : ''}`

    return (
        <button
            type="button"
            className={cn(itemStyles.item, styles.row, compact && itemStyles.compact)}
            onClick={onOpen}
            aria-label={compact ? 'Архив' : undefined}
            title={compact ? 'Архив' : undefined}
        >
            <span className={itemStyles.avatar}>
                <span className={styles.tile}>
                    <Icon name="archive" size={22} />
                </span>
                {compact && unread > 0 && (
                    <span className={cn(itemStyles.badge, itemStyles.mutedBadge)}>
                        {unreadLabel(unread)}
                    </span>
                )}
            </span>
            {!compact && (
                <>
                    <span className={itemStyles.texts}>
                        <span className={itemStyles.name}>Архив</span>
                        <span className={itemStyles.preview}>
                            {count} {plural(chats.length, ['чат', 'чата', 'чатов'])}
                        </span>
                    </span>
                    {unread > 0 && (
                        <span className={itemStyles.meta}>
                            <Tag variant="muted">{unreadLabel(unread)}</Tag>
                        </span>
                    )}
                </>
            )}
        </button>
    )
}

// шапка открытого архива: назад к основному списку
export function ArchiveHeader({ compact, onBack }: { compact: boolean; onBack: () => void }) {
    return (
        <button
            type="button"
            className={cn(styles.header, compact && styles.headerCompact)}
            onClick={onBack}
            aria-label="Назад к чатам"
        >
            <Icon name="arrow-left" size={20} color="currentColor" />
            {!compact && <span className={styles.title}>К чатам</span>}
        </button>
    )
}
