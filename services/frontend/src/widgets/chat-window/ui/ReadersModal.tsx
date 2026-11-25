import { useState } from 'react'

import { useReaders } from '@/entities/message'
import { useProfileCard, UserRow } from '@/entities/user'
import { Button, Modal, Skeleton } from '@/shared/ui'

import styles from './ReadersModal.module.scss'

interface Props {
    conversationId: string
    messageId: string | null
    onClose: () => void
}

// окно со списком участников группы, прочитавших сообщение
export function ReadersModal({ conversationId, messageId, onClose }: Props) {
    // сохраняем последний id на время анимации закрытия модалки, чтобы не мелькал скелетон
    const [shownId, setShownId] = useState<string | null>(messageId)
    if (messageId && messageId !== shownId) {
        setShownId(messageId)
    }

    const readers = useReaders(conversationId, shownId, !!messageId)
    const openProfile = useProfileCard((state) => state.open)
    const ids = readers.data ?? []

    return (
        <Modal open={!!messageId} onClose={onClose} title="Прочитали" width={400}>
            {readers.isPending ? (
                <div className={styles.list} aria-busy="true">
                    {[0, 1, 2].map((i) => (
                        <div key={i} className={styles.row}>
                            <Skeleton shape="circle" width={40} />
                            <span className={styles.texts}>
                                <Skeleton shape="line" width={140} height={16} />
                                <Skeleton shape="line" width={90} height={14} />
                            </span>
                        </div>
                    ))}
                </div>
            ) : readers.isError ? (
                <div className={styles.state}>
                    <p>Не удалось загрузить список</p>
                    <Button variant="secondary" onClick={() => void readers.refetch()}>
                        Повторить
                    </Button>
                </div>
            ) : ids.length === 0 ? (
                <div className={styles.state}>
                    <p>Пока никто не прочитал</p>
                </div>
            ) : (
                <div className={styles.list}>
                    {ids.map((userId) => (
                        <UserRow
                            key={userId}
                            userId={userId}
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
