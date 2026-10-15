import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'

import { findDirectWith } from '@/entities/conversation'
import {
    useBlockStatus,
    useMe,
    usePresence,
    useProfileCard,
    UserAvatar,
    useUser,
} from '@/entities/user'
import { useBlockUser } from '@/features/block-user'
import { chatPath, newChatPath } from '@/shared/config'
import { formatDate, formatLastSeen } from '@/shared/lib'
import type { InfoItem } from '@/shared/ui'
import { Button, ConfirmModal, Icon, InfoList, Loader, Modal, Status } from '@/shared/ui'

import styles from './UserProfileModal.module.scss'

// модалка профиля: открывается из поиска, шапки чата и меню профиля (стор useProfileCard);
// свой профиль - без кнопок «Написать» и «Заблокировать»
export function UserProfileModal() {
    const userId = useProfileCard((state) => state.userId)
    const close = useProfileCard((state) => state.close)
    // при закрытии содержимое остаётся прежним, пока играет анимация
    const [shownId, setShownId] = useState(userId)
    if (userId && userId !== shownId) {
        setShownId(userId)
    }

    return shownId ? (
        <ProfileContent key={shownId} userId={shownId} open={!!userId} onClose={close} />
    ) : null
}

interface ContentProps {
    userId: string
    open: boolean
    onClose: () => void
}

function ProfileContent({ userId, open, onClose }: ContentProps) {
    const me = useMe()
    const isMe = me.data?.user_id === userId
    const other = useUser(isMe ? null : userId)
    const profile = isMe ? me.data : other.data
    const presence = usePresence(isMe ? null : userId)
    const blockStatus = useBlockStatus(isMe ? null : userId)
    const { block, unblock, pending } = useBlockUser(userId)
    const [confirmBlock, setConfirmBlock] = useState(false)
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    if (!profile) {
        return (
            <Modal open={open} onClose={onClose} aria-label="Профиль">
                <div className={styles.loading}>
                    {other.isError ? (
                        <p className={styles.muted}>Профиль недоступен</p>
                    ) : (
                        <Loader />
                    )}
                </div>
            </Modal>
        )
    }

    const online = !isMe && (presence?.online ?? profile.online)
    const subtitle = isMe
        ? `@${profile.username}`
        : online
          ? 'в сети'
          : formatLastSeen(presence?.lastSeenAt ?? profile.last_seen_at)
    const blockedByMe = blockStatus.data?.blocked_by_me ?? false

    const info: InfoItem[] = [
        { icon: 'shape-user', label: 'Имя пользователя', value: `@${profile.username}` },
        { icon: 'shape-plus', label: 'Аккаунт создан', value: formatDate(profile.created_at) },
    ]
    if (profile.bio) {
        info.push({ icon: 'shape-message', label: 'О себе', value: profile.bio })
    }

    function write() {
        const direct = findDirectWith(queryClient, userId)
        onClose()
        navigate(direct ? chatPath(direct.id) : newChatPath(userId))
    }

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-label={`Профиль: ${profile.display_name}`}
            headerStart={
                profile.status && (
                    <Status
                        icon={profile.status.icon}
                        text={profile.status.text}
                        background={profile.status.background}
                        foreground={profile.status.foreground}
                        expiresAt={profile.status.expires_at}
                    />
                )
            }
        >
            <div className={styles.head}>
                <UserAvatar
                    src={profile.avatar_url}
                    shape={profile.avatar_shape}
                    size={96}
                    online={online}
                />
                <div className={styles.names}>
                    <h2 className={styles.name}>{profile.display_name}</h2>
                    <span className={styles.muted}>{subtitle}</span>
                </div>
            </div>

            <div className={styles.content}>
                {!isMe && (
                    <div className={styles.actions}>
                        <Button variant="secondary" aria-label="Написать сообщение" onClick={write}>
                            <Icon name="message" size={22} color="inherit" />
                        </Button>
                        <Button
                            variant="danger"
                            aria-label={blockedByMe ? 'Разблокировать' : 'Заблокировать'}
                            disabled={pending || blockStatus.isPending}
                            onClick={() => (blockedByMe ? unblock.mutate() : setConfirmBlock(true))}
                        >
                            <Icon name="block" size={22} color="inherit" />
                            {blockedByMe && <span>Разблокировать</span>}
                        </Button>
                    </div>
                )}

                <InfoList items={info} />
            </div>

            <ConfirmModal
                open={confirmBlock}
                onClose={() => setConfirmBlock(false)}
                onConfirm={() => {
                    setConfirmBlock(false)
                    block.mutate()
                }}
                confirmLabel="Заблокировать"
                danger
            >
                {profile.display_name} не сможет писать вам и не увидит ваш аватар, статус и время в
                сети
            </ConfirmModal>
        </Modal>
    )
}
