import { useNavigate } from 'react-router'

import type { GroupPreview } from '@/entities/conversation'
import { useJoinGroup, useRequestJoin } from '@/features/join-group'
import { chatLinkText, chatPath } from '@/shared/config'
import type { InfoItem } from '@/shared/ui'
import { Button, InfoList, Modal, Status } from '@/shared/ui'

import { GroupHead, statusProps } from './GroupHead'
import styles from './GroupInfo.module.scss'

interface Props {
    group: GroupPreview
    open: boolean
    onClose: () => void
}

// группа, в которой я не состою: открытая - «Вступить», закрытая по ссылке - «Отправить запрос»
export function PreviewInfo({ group, open, onClose }: Props) {
    const join = useJoinGroup(onClose)
    const request = useRequestJoin()
    const navigate = useNavigate()
    const sent = group.request_status === 'pending'

    const info: InfoItem[] = []
    if (group.description) {
        info.push({ icon: 'shape-message', label: 'Описание', value: group.description })
    }
    if (group.visibility === 'public' && group.slug) {
        info.push({ icon: 'link', label: 'Ссылка на чат', value: chatLinkText(group.slug) })
    }

    let action
    if (group.is_member) {
        action = (
            <Button
                onClick={() => {
                    onClose()
                    navigate(chatPath(group.id))
                }}
            >
                Открыть чат
            </Button>
        )
    } else if (group.visibility === 'public') {
        action = (
            <Button disabled={join.isPending} onClick={() => join.mutate(group.id)}>
                Вступить
            </Button>
        )
    } else {
        action = (
            <Button
                disabled={sent || !group.code || request.isPending}
                onClick={() =>
                    group.code && request.mutate({ groupId: group.id, code: group.code })
                }
            >
                {sent ? 'Запрос отправлен' : 'Отправить запрос'}
            </Button>
        )
    }

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-label={`Группа: ${group.name}`}
            headerStart={group.status && <Status {...statusProps(group.status)} />}
        >
            <GroupHead
                name={group.name}
                avatarUrl={group.avatar_url}
                shape={group.avatar_shape}
                membersCount={group.members_count}
            />
            {info.length > 0 && <InfoList items={info} />}
            {group.visibility === 'private' && !group.is_member && (
                <p className={styles.hint}>
                    Это закрытый чат: вступить можно с одобрения администратора
                </p>
            )}
            {action}
        </Modal>
    )
}
