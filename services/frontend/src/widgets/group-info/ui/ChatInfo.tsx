import { useState } from 'react'

import { useConversation, useGroupMembers } from '@/entities/conversation'
import { ChatLink, inviteCodeOf } from '@/features/group/invite-link'
import { AddMembersPicker } from '@/features/group/members'
import { GroupSettingsForm } from '@/features/group/settings'
import { AvatarShapeEditor } from '@/features/set-avatar-shape'
import { StatusButton, StatusEditor } from '@/features/set-status'
import { formatDate } from '@/shared/lib'
import type { InfoItem } from '@/shared/ui'
import { Button, Icon, InfoList, Loader, Modal, Status } from '@/shared/ui'

import { GroupHead, statusProps } from './GroupHead'
import styles from './GroupInfo.module.scss'
import { MembersScreen } from './MembersScreen'

type Screen = 'info' | 'settings' | 'status' | 'shape' | 'members' | 'add'

const TITLES: Partial<Record<Screen, string>> = {
    status: 'Статус группы',
    shape: 'Фигура',
    members: 'Участники',
    add: 'Добавить участников',
}

// «назад» ведёт на уровень выше
const PARENT: Record<Exclude<Screen, 'info'>, Screen> = {
    settings: 'info',
    status: 'settings',
    shape: 'settings',
    members: 'info',
    add: 'members',
}

interface Props {
    id: string
    open: boolean
    onClose: () => void
}

// своя группа: сведения, настройки (админ), статус группы, участники и их добавление - экраны одного окна
export function ChatInfo({ id, open, onClose }: Props) {
    const chat = useConversation(id)
    const [screen, setScreen] = useState<Screen>('info')
    const data = chat.data
    const isAdmin = data?.my_role === 'admin'
    // кто уже в группе - для выбора новых участников
    const members = useGroupMembers(id, screen === 'add')

    if (!data) {
        return (
            <Modal open={open} onClose={onClose} aria-label="Группа">
                <div className={styles.loading}>
                    {chat.isError ? <p className={styles.muted}>Чат недоступен</p> : <Loader />}
                </div>
            </Modal>
        )
    }

    const name = data.name ?? 'Группа'
    // key={screen} у окна: каждый экран - новое окно со своей анимацией появления, как в настройках
    const back =
        screen === 'info' ? null : (
            <button
                type="button"
                className={styles.back}
                aria-label="Назад"
                onClick={() => setScreen(PARENT[screen])}
            >
                <Icon name="arrow-left" size={24} color="inherit" />
            </button>
        )

    if (screen === 'settings' && isAdmin) {
        return (
            <Modal
                key={screen}
                open={open}
                onClose={onClose}
                aria-label={`Настройки группы: ${name}`}
                headerStart={
                    <span className={styles.headerStart}>
                        {back}
                        <StatusButton
                            target={{ kind: 'group', id }}
                            onEdit={() => setScreen('status')}
                        />
                    </span>
                }
            >
                <GroupSettingsForm
                    chat={data}
                    onPickShape={() => setScreen('shape')}
                    onSaved={() => setScreen('info')}
                    onDeleted={onClose}
                />
            </Modal>
        )
    }

    if (screen === 'status' && isAdmin) {
        return (
            <Modal
                key={screen}
                open={open}
                onClose={onClose}
                title={TITLES.status}
                headerStart={back}
            >
                <StatusEditor target={{ kind: 'group', id }} onDone={() => setScreen('settings')} />
            </Modal>
        )
    }

    if (screen === 'shape' && isAdmin) {
        return (
            <Modal
                key={screen}
                open={open}
                onClose={onClose}
                title={TITLES.shape}
                headerStart={back}
            >
                <AvatarShapeEditor
                    target={{ kind: 'group', id }}
                    onDone={() => setScreen('settings')}
                />
            </Modal>
        )
    }

    if (screen === 'members') {
        return (
            <Modal
                key={screen}
                open={open}
                onClose={onClose}
                title={TITLES.members}
                headerStart={back}
                className={styles.tall}
            >
                <MembersScreen chat={data} onAdd={() => setScreen('add')} />
            </Modal>
        )
    }

    if (screen === 'add' && isAdmin) {
        return (
            <Modal
                key={screen}
                open={open}
                onClose={onClose}
                title={TITLES.add}
                headerStart={back}
                className={styles.tall}
            >
                <AddMembersPicker
                    groupId={id}
                    memberIds={(members.data ?? []).map((m) => m.user_id)}
                    onDone={() => setScreen('members')}
                />
            </Modal>
        )
    }

    const info: InfoItem[] = [
        { icon: 'shape-plus', label: 'Чат создан', value: formatDate(data.created_at) },
    ]
    if (data.description) {
        info.push({ icon: 'shape-message', label: 'Описание', value: data.description })
    }
    // ссылка открытой группы видна всем, закрытой - только админам
    if (inviteCodeOf(data)) {
        info.push({ icon: 'link', label: 'Ссылка на чат', value: <ChatLink chat={data} /> })
    }

    return (
        <Modal
            key={screen}
            open={open}
            onClose={onClose}
            aria-label={`Группа: ${name}`}
            headerStart={data.status && <Status {...statusProps(data.status)} />}
        >
            <GroupHead
                name={name}
                avatarUrl={data.avatar_url}
                shape={data.avatar_shape}
                membersCount={data.members_count}
            />
            <div className={styles.content}>
                <div className={styles.actions}>
                    {isAdmin && (
                        <Button
                            variant="secondary"
                            icon="settings"
                            aria-label="Настройки группы"
                            onClick={() => setScreen('settings')}
                        />
                    )}
                    <Button
                        variant="secondary"
                        icon="users"
                        aria-label="Участники"
                        onClick={() => setScreen('members')}
                    />
                </div>
                <InfoList items={info} />
            </div>
        </Modal>
    )
}
