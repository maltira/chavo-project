import { useState } from 'react'

import { useDirectPeers } from '@/entities/conversation'
import { UserRow } from '@/entities/user'
import { Button, IconButton } from '@/shared/ui'

import { useAddMembers } from '../model/useMemberActions'
import styles from './AddMembersPicker.module.scss'

interface Props {
    groupId: string
    // кто уже в группе: их не предлагаем
    memberIds: string[]
    // добавлены - вернуться к списку участников
    onDone: () => void
}

// выбор новых участников из моих собеседников по личным чатам
export function AddMembersPicker({ groupId, memberIds, onDone }: Props) {
    const peers = useDirectPeers().filter((id) => !memberIds.includes(id))
    const [selected, setSelected] = useState<string[]>([])
    const add = useAddMembers(groupId, onDone)

    const toggle = (userId: string) =>
        setSelected((ids) =>
            ids.includes(userId) ? ids.filter((id) => id !== userId) : [...ids, userId],
        )

    return (
        <>
            <div className={styles.body}>
                <p className={styles.label}>Выберите, кого добавить:</p>
                {peers.length > 0 ? (
                    <div className={styles.list}>
                        {peers.map((userId) => {
                            const chosen = selected.includes(userId)
                            return (
                                <UserRow
                                    key={userId}
                                    userId={userId}
                                    aside={(profile) => (
                                        <IconButton
                                            icon={chosen ? 'check' : 'add'}
                                            size={40}
                                            aria-label={`${chosen ? 'Не добавлять' : 'Добавить'} ${profile?.display_name ?? ''}`.trim()}
                                            aria-pressed={chosen}
                                            className={styles.toggle}
                                            onClick={() => toggle(userId)}
                                        />
                                    )}
                                />
                            )
                        })}
                    </div>
                ) : (
                    <p className={styles.hint}>
                        Позвать можно тех, с кем вы переписываетесь, - все они уже в группе
                    </p>
                )}
            </div>

            <div className={styles.footer}>
                <p className={styles.label}>Выбрано: {selected.length}</p>
                <Button
                    disabled={selected.length === 0 || add.isPending}
                    onClick={() => add.mutate(selected)}
                >
                    Добавить
                </Button>
            </div>
        </>
    )
}
