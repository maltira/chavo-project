import { useState } from 'react'

import type { MessageReaction } from '@/entities/message'
import { useProfileCard, UserRow } from '@/entities/user'
import { cn } from '@/shared/lib'
import { Modal } from '@/shared/ui'

import styles from './ReactorsModal.module.scss'

interface Props {
    reactions: MessageReaction[] | null
    initialEmoji?: string
    onClose: () => void
}

// окно со списком пользователей, поставивших реакции на сообщение
export function ReactorsModal({ reactions, initialEmoji, onClose }: Props) {
    const [shown, setShown] = useState<MessageReaction[]>(() => reactions ?? [])
    const [activeTab, setActiveTab] = useState<string>(initialEmoji ?? 'all')
    const [prevReactions, setPrevReactions] = useState(reactions)
    const [prevInitial, setPrevInitial] = useState(initialEmoji)

    if (reactions !== prevReactions || initialEmoji !== prevInitial) {
        setPrevReactions(reactions)
        setPrevInitial(initialEmoji)
        if (reactions) {
            setShown(reactions)
            setActiveTab(initialEmoji ?? 'all')
        }
    }

    const openProfile = useProfileCard((state) => state.open)
    const totalCount = shown.reduce((sum, r) => sum + r.count, 0)
    const effectiveTab =
        activeTab === 'all' || shown.some((r) => r.emoji === activeTab) ? activeTab : 'all'

    const entries: { userId: string; emoji: string }[] = []
    for (const r of shown) {
        if (effectiveTab !== 'all' && r.emoji !== effectiveTab) {
            continue
        }
        for (const userId of r.user_ids) {
            entries.push({ userId, emoji: r.emoji })
        }
    }

    return (
        <Modal open={!!reactions} onClose={onClose} title="Реакции" width={400}>
            {shown.length > 0 && (
                <div className={styles.tabs} role="tablist" aria-label="Фильтр реакций">
                    <button
                        type="button"
                        role="tab"
                        aria-selected={effectiveTab === 'all'}
                        className={cn(styles.tab, effectiveTab === 'all' && styles.tabActive)}
                        onClick={() => setActiveTab('all')}
                    >
                        <span>Все</span>
                        <span className={styles.tabCount}>{totalCount}</span>
                    </button>
                    {shown.map((r) => (
                        <button
                            key={r.emoji}
                            type="button"
                            role="tab"
                            aria-selected={effectiveTab === r.emoji}
                            className={cn(
                                styles.tab,
                                effectiveTab === r.emoji && styles.tabActive,
                            )}
                            onClick={() => setActiveTab(r.emoji)}
                        >
                            <span>{r.emoji}</span>
                            <span className={styles.tabCount}>{r.count}</span>
                        </button>
                    ))}
                </div>
            )}

            {entries.length === 0 ? (
                <div className={styles.empty}>Реакций пока нет</div>
            ) : (
                <div className={styles.list}>
                    {entries.map(({ userId, emoji }) => (
                        <UserRow
                            key={`${emoji}:${userId}`}
                            userId={userId}
                            aside={
                                effectiveTab === 'all' ? (
                                    <span className={styles.rowEmoji} aria-hidden="true">
                                        {emoji}
                                    </span>
                                ) : undefined
                            }
                            onClick={() => {
                                onClose()
                                openProfile(userId)
                            }}
                        />
                    ))}
                </div>
            )}
        </Modal>
    )
}
