import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'

import { findDirectWith } from '@/entities/conversation'
import {
    useAvatarPhotos,
    useBlockStatus,
    useMe,
    useMyAvatar,
    usePresence,
    useProfileCard,
    UserAvatar,
    useUser,
} from '@/entities/user'
import { useBlockUser } from '@/features/block-user'
import { ProfileEditForm } from '@/features/profile-edit'
import { AvatarShapeEditor } from '@/features/set-avatar-shape'
import { StatusButton, StatusEditor } from '@/features/set-status'
import { chatPath, newChatPath } from '@/shared/config'
import { formatDate, formatLastSeen } from '@/shared/lib'
import type { InfoItem } from '@/shared/ui'
import {
    Button,
    ConfirmModal,
    Icon,
    InfoList,
    Loader,
    Modal,
    PhotoViewer,
    ProfileCover,
    Status,
} from '@/shared/ui'

import styles from './UserProfileModal.module.scss'

// модалка профиля: открывается из поиска, шапки чата и меню профиля (стор useProfileCard);
// свой профиль - без кнопок «Написать» и «Заблокировать», зато со статусом, фигурой и редактированием
export function UserProfileModal() {
    const userId = useProfileCard((state) => state.userId)
    const close = useProfileCard((state) => state.close)
    // при закрытии содержимое остаётся прежним, пока играет анимация
    const [shownId, setShownId] = useState(userId)
    if (userId && userId !== shownId) {
        setShownId(userId)
    }
    // каждое открытие - с самого профиля, а не со статуса или правки, где окно закрыли
    const open = !!userId
    const [session, setSession] = useState(0)
    const [wasOpen, setWasOpen] = useState(open)
    if (open !== wasOpen) {
        setWasOpen(open)
        if (open) {
            setSession((n) => n + 1)
        }
    }

    return shownId ? (
        <ProfileContent
            key={`${session}:${shownId}`}
            userId={shownId}
            open={open}
            onClose={close}
        />
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
    // свой профиль: просмотр или редактирование, и окно выбора фигуры за аватаром
    const [editing, setEditing] = useState(false)
    const [pickingShape, setPickingShape] = useState(false)
    // фото на весь экран с историей аватарок; своими фото можно управлять оттуда
    const [viewing, setViewing] = useState(false)
    const myPhoto = useMyAvatar()
    const photos = useAvatarPhotos(userId, viewing)
    // редактор статуса - экран в этом же окне, со стрелкой «назад» к профилю
    const [editingStatus, setEditingStatus] = useState(false)
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    // все фото удалены - просмотру показывать нечего, и при новой аватарке он сам не откроется
    if (viewing && profile && !profile.avatar_url) {
        setViewing(false)
    }

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

    if (profile.is_deleted) {
        return (
            <Modal
                key="deleted-profile"
                open={open}
                onClose={onClose}
                aria-label="Удалённый аккаунт"
            >
                <div className={styles.head}>
                    <span className={styles.avatarWrap}>
                        <UserAvatar size={96} />
                    </span>
                    <div className={styles.names}>
                        <h2 className={styles.name}>Удалённый аккаунт</h2>
                        <span className={styles.muted}>был(а) давно</span>
                    </div>
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

    // разные key у окон профиля, статуса и оформления: переход между ними - новое окно со своей анимацией
    if (isMe && pickingShape) {
        return (
            <Modal
                key="shape"
                open={open}
                onClose={onClose}
                title="Оформление"
                headerStart={
                    <button
                        type="button"
                        className={styles.back}
                        aria-label="Назад"
                        onClick={() => setPickingShape(false)}
                    >
                        <Icon name="arrow-left" size={24} color="inherit" />
                    </button>
                }
            >
                <AvatarShapeEditor onDone={() => setPickingShape(false)} />
            </Modal>
        )
    }

    if (isMe && editingStatus) {
        return (
            <Modal
                key="status"
                open={open}
                onClose={onClose}
                title="Статус"
                headerStart={
                    <button
                        type="button"
                        className={styles.back}
                        aria-label="Назад"
                        onClick={() => setEditingStatus(false)}
                    >
                        <Icon name="arrow-left" size={24} color="inherit" />
                    </button>
                }
            >
                <StatusEditor onDone={() => setEditingStatus(false)} />
            </Modal>
        )
    }

    function write() {
        const direct = findDirectWith(queryClient, userId)
        onClose()
        navigate(direct ? chatPath(direct.id) : newChatPath(userId))
    }

    return (
        <Modal
            key="profile"
            open={open}
            onClose={onClose}
            aria-label={`Профиль: ${profile.display_name}`}
            cover={
                !(isMe && editing) && profile.profile_cover ? (
                    <ProfileCover
                        color={profile.profile_cover.color}
                        pattern={profile.profile_cover.pattern}
                    />
                ) : undefined
            }
            headerStart={
                // свой статус - кнопка: по нажатию редактор статуса
                isMe ? (
                    <StatusButton animate onEdit={() => setEditingStatus(true)} />
                ) : (
                    profile.status && (
                        <Status
                            icon={profile.status.icon}
                            text={profile.status.text}
                            background={profile.status.background}
                            foreground={profile.status.foreground}
                            expiresAt={profile.status.expires_at}
                            animate
                        />
                    )
                )
            }
        >
            {isMe && editing ? (
                <ProfileEditForm
                    profile={profile}
                    onDone={() => setEditing(false)}
                    onPickShape={() => setPickingShape(true)}
                />
            ) : (
                <>
                    <div className={styles.head}>
                        <span className={styles.avatarWrap}>
                            {profile.avatar_url ? (
                                // аватар с фото открывается на весь экран
                                <button
                                    type="button"
                                    className={styles.avatarButton}
                                    aria-label="Открыть фото"
                                    onClick={() => setViewing(true)}
                                >
                                    <UserAvatar
                                        src={profile.avatar_url}
                                        shape={profile.avatar_shape}
                                        size={96}
                                    />
                                </button>
                            ) : (
                                <UserAvatar
                                    src={profile.avatar_url}
                                    shape={profile.avatar_shape}
                                    size={96}
                                />
                            )}
                        </span>
                        {viewing && profile.avatar_url && (
                            <PhotoViewer
                                // пока история грузится, показывается текущее фото
                                photos={
                                    photos.data?.length
                                        ? photos.data.map((p) => ({ id: p.id, src: p.avatar_url }))
                                        : [{ id: 'current', src: profile.avatar_url }]
                                }
                                title={profile.display_name}
                                onClose={() => setViewing(false)}
                                onSelect={
                                    isMe ? (photo) => myPhoto.select.mutate(photo.id) : undefined
                                }
                                onDelete={
                                    isMe
                                        ? (photo) =>
                                              // последнее фото убрано - историю смотреть больше не на что
                                              photo.id === 'current'
                                                  ? myPhoto.remove.mutate()
                                                  : myPhoto.removePhoto.mutate(photo.id)
                                        : undefined
                                }
                                busy={myPhoto.pending}
                            />
                        )}
                        <div className={styles.names}>
                            <h2 className={styles.name}>{profile.display_name}</h2>
                            <span className={styles.muted}>{subtitle}</span>
                        </div>
                    </div>

                    <div className={styles.content}>
                        {!isMe && (
                            <div className={styles.actions}>
                                <Button
                                    variant="secondary"
                                    aria-label="Написать сообщение"
                                    onClick={write}
                                >
                                    <Icon name="message" size={22} color="inherit" />
                                </Button>
                                <Button
                                    variant="danger"
                                    aria-label={blockedByMe ? 'Разблокировать' : 'Заблокировать'}
                                    disabled={pending || blockStatus.isPending}
                                    onClick={() =>
                                        blockedByMe ? unblock.mutate() : setConfirmBlock(true)
                                    }
                                >
                                    <Icon name="block" size={22} color="inherit" />
                                    {blockedByMe && <span>Разблокировать</span>}
                                </Button>
                            </div>
                        )}

                        <InfoList items={info} />

                        {isMe && (
                            <Button variant="secondary" onClick={() => setEditing(true)}>
                                Редактировать
                            </Button>
                        )}
                    </div>
                </>
            )}

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
                {profile.display_name} не сможет писать вам и не увидит ваш аватар, обложку, статус
                и время в сети
            </ConfirmModal>
        </Modal>
    )
}
