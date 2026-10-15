import { useState } from 'react'
import { useNavigate } from 'react-router'

import { useGroupCard, useMembers } from '@/entities/conversation'
import type { PublicGroup } from '@/entities/conversation'
import { UserAvatar } from '@/entities/user'
import { useJoinGroup } from '@/features/join-group'
import { chatPath } from '@/shared/config'
import { plural } from '@/shared/lib'
import { Button, InfoList, Modal, Status } from '@/shared/ui'

import styles from './GroupPreviewModal.module.scss'

// открытая группа из поиска: описание и «Вступить» (или «Открыть чат», если уже в ней)
export function GroupPreviewModal() {
    const group = useGroupCard((state) => state.group)
    const close = useGroupCard((state) => state.close)

    const [shown, setShown] = useState(group)
    if (group && group !== shown) {
        setShown(group)
    }

    return shown ? (
        <GroupContent key={shown.id} group={shown} open={!!group} onClose={close} />
    ) : null
}

interface ContentProps {
    group: PublicGroup
    open: boolean
    onClose: () => void
}

function GroupContent({ group, open, onClose }: ContentProps) {
    const join = useJoinGroup(onClose)
    const navigate = useNavigate()

    const members = useMembers(group.members_count === null ? group.id : null)
    const count = group.members_count ?? members.data?.length

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-label={`Группа: ${group.name}`}
            headerStart={
                group.status && (
                    <Status
                        icon={group.status.icon}
                        text={group.status.text}
                        background={group.status.background}
                        foreground={group.status.foreground}
                        expiresAt={group.status.expires_at}
                    />
                )
            }
        >
            <div className={styles.head}>
                <UserAvatar src={group.avatar_url} shape={group.avatar_shape} size={96} />
                <div className={styles.names}>
                    <h2 className={styles.name}>{group.name}</h2>
                    <span className={styles.muted}>
                        {count === undefined
                            ? '\u00a0'
                            : `${count} ${plural(count, ['участник', 'участника', 'участников'])}`}
                    </span>
                </div>
            </div>

            {group.description && (
                <InfoList
                    items={[{ icon: 'shape-message', label: 'Описание', value: group.description }]}
                />
            )}

            {group.is_member ? (
                <Button
                    onClick={() => {
                        onClose()
                        navigate(chatPath(group.id))
                    }}
                >
                    Открыть чат
                </Button>
            ) : (
                <Button disabled={join.isPending} onClick={() => join.mutate(group.id)}>
                    Вступить
                </Button>
            )}
        </Modal>
    )
}
