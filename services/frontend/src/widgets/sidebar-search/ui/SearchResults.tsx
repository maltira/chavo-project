import { useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useNavigate } from 'react-router'

import {
    previewFromSearch,
    SAVED_NAME,
    SavedAvatar,
    useConversations,
    useGroupCard,
    useGroupSearch,
} from '@/entities/conversation'
import type { Profile } from '@/entities/user'
import {
    useMe,
    usePresence,
    useProfileCard,
    useRecentUsers,
    UserAvatar,
    userKey,
    useUser,
    useUserSearch,
} from '@/entities/user'
import { chatPath } from '@/shared/config'
import { plural, useDebouncedValue } from '@/shared/lib'
import { SkeletonRow } from '@/shared/ui'

import { matchChat, normalizeQuery } from '../lib/match'
import styles from './SearchResults.module.scss'
import { Icon } from '@/shared/ui'

// сколько строк-заглушек показывать, пока идёт поиск
const SKELETON_ROWS = 3

interface Props {
    query: string
    close: () => void // выйти из поиска (после перехода в чат)
}

// поиск в сайдбаре: недавно открытые профили, люди и открытые группы с сервера, мои чаты
export function SearchResults({ query, close }: Props) {
    const q = useDebouncedValue(normalizeQuery(query))
    const me = useMe()
    const users = useUserSearch(q)
    const groups = useGroupSearch(q)
    const chats = useConversations()
    const recent = useRecentUsers((state) => state.ids)
    const clearRecent = useRecentUsers((state) => state.clear)
    const openProfile = useProfileCard((state) => state.open)
    const openGroup = useGroupCard((state) => state.openPreview)
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    const people = (users.data ?? []).filter((u) => u.user_id !== me.data?.user_id)
    const publicGroups = groups.data ?? []
    // имена собеседников берутся из уже загруженных профилей (их подгрузил список чатов)
    const myChats = (chats.data ?? []).filter((chat) =>
        matchChat(
            chat,
            q,
            chat.peer_id ? queryClient.getQueryData<Profile>(userKey(chat.peer_id)) : undefined,
        ),
    )
    const found = people.length + publicGroups.length
    const searching = (users.isFetching || groups.isFetching) && found === 0

    function openChat(id: string) {
        close()
        navigate(chatPath(id))
    }

    return (
        <div className={styles.root}>
            {recent.length > 0 && (
                <section className={styles.section} aria-label="Недавние">
                    <div className={styles.heading}>
                        <h3 className={styles.title}>Недавние</h3>
                        <button type="button" className={styles.clear} onClick={clearRecent}>
                            Очистить
                        </button>
                    </div>
                    <div className={styles.recent}>
                        {recent.map((id) => (
                            <RecentAvatar key={id} userId={id} onOpen={() => openProfile(id)} />
                        ))}
                    </div>
                </section>
            )}

            {!q ? (
                recent.length === 0 && (
                    <div className={styles.hintBlock}>
                        <Icon name="search" size={32} />
                        <p className={styles.hint}>Введите больше символов для поиска</p>
                    </div>
                )
            ) : (
                <>
                    {found > 0 && (
                        <section className={styles.section} aria-label="Общий поиск">
                            <h3 className={styles.title}>Глобальный поиск</h3>
                            {people.map((user) => (
                                <UserRow
                                    key={user.user_id}
                                    user={user}
                                    onOpen={() => openProfile(user.user_id)}
                                />
                            ))}
                            {publicGroups.map((group) => (
                                <Row
                                    key={group.id}
                                    avatar={
                                        <UserAvatar
                                            src={group.avatar_url}
                                            shape={group.avatar_shape}
                                        />
                                    }
                                    title={group.name}
                                    subtitle={`${group.members_count} ${plural(group.members_count, ['участник', 'участника', 'участников'])}`}
                                    onClick={() => openGroup(previewFromSearch(group))}
                                />
                            ))}
                        </section>
                    )}
                    {searching && (
                        <div aria-busy="true" aria-label="Ищем">
                            {Array.from({ length: SKELETON_ROWS }, (_, i) => (
                                <SkeletonRow key={i} />
                            ))}
                        </div>
                    )}

                    {myChats.length > 0 && (
                        <section className={styles.section} aria-label="Мои чаты">
                            <h3 className={styles.title}>Мои чаты</h3>
                            {myChats.map((chat) =>
                                chat.is_saved ? (
                                    <Row
                                        key={chat.id}
                                        avatar={<SavedAvatar />}
                                        title={SAVED_NAME}
                                        subtitle="Сообщения для себя"
                                        onClick={() => openChat(chat.id)}
                                    />
                                ) : chat.conversation_type === 'direct' && chat.peer_id ? (
                                    <PeerRow
                                        key={chat.id}
                                        userId={chat.peer_id}
                                        onOpen={() => openChat(chat.id)}
                                    />
                                ) : (
                                    <Row
                                        key={chat.id}
                                        avatar={
                                            <UserAvatar
                                                src={chat.avatar_url}
                                                shape={chat.avatar_shape}
                                            />
                                        }
                                        title={chat.name ?? 'Группа'}
                                        subtitle="Группа"
                                        onClick={() => openChat(chat.id)}
                                    />
                                ),
                            )}
                        </section>
                    )}

                    {!searching && found === 0 && myChats.length === 0 && (
                        <div className={styles.hintBlock}>
                            <Icon name="search" size={32} />
                            <p className={styles.hint}>По вашему запросу ничего не найдено</p>
                        </div>
                    )}
                </>
            )}
        </div>
    )
}

interface RowProps {
    avatar: ReactNode
    title: string
    subtitle: string
    onClick: () => void
}

function Row({ avatar, title, subtitle, onClick }: RowProps) {
    return (
        <button type="button" className={styles.row} onClick={onClick}>
            {avatar}
            <span className={styles.texts}>
                <span className={styles.name}>{title}</span>
                <span className={styles.subtitle}>{subtitle}</span>
            </span>
        </button>
    )
}

function UserRow({ user, onOpen }: { user: Profile; onOpen: () => void }) {
    const presence = usePresence(user.user_id)
    return (
        <Row
            avatar={
                <UserAvatar
                    src={user.avatar_url}
                    shape={user.avatar_shape}
                    online={presence?.online ?? user.online}
                />
            }
            title={user.display_name}
            subtitle={`@${user.username}`}
            onClick={onOpen}
        />
    )
}

function PeerRow({ userId, onOpen }: { userId: string; onOpen: () => void }) {
    const user = useUser(userId)
    return user.data ? <UserRow user={user.data} onOpen={onOpen} /> : null
}

function RecentAvatar({ userId, onOpen }: { userId: string; onOpen: () => void }) {
    const user = useUser(userId)
    const presence = usePresence(userId)
    const profile = user.data
    return (
        <button
            type="button"
            className={styles.recentItem}
            aria-label={profile?.display_name ?? 'Профиль'}
            title={profile?.display_name}
            onClick={onOpen}
        >
            <UserAvatar
                src={profile?.avatar_url}
                shape={profile?.avatar_shape}
                online={presence?.online ?? profile?.online ?? false}
            />
        </button>
    )
}
