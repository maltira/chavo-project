import type { MouseEvent, ReactNode } from 'react'

import { cn } from '@/shared/lib'
import { Skeleton } from '@/shared/ui'

import type { Profile } from '../model/types'
import { useUser } from '../model/useUser'
import { UserAvatar } from './UserAvatar'
import styles from './UserRow.module.scss'

interface Props {
    userId: string
    // справа: тег «Админ», кнопки действий; функция - когда подписи кнопок зависят от имени
    aside?: ReactNode | ((profile: Profile | undefined) => ReactNode)
    // клик по аватару и имени (например, профиль)
    onClick?: () => void
    onContextMenu?: (event: MouseEvent) => void
    className?: string
}

// человек в списке: аватар 40, имя и @username; профиль подгружается сам
export function UserRow({ userId, aside, onClick, onContextMenu, className }: Props) {
    const user = useUser(userId)
    const profile = user.data

    const body = profile ? (
        <>
            <UserAvatar src={profile.avatar_url} shape={profile.avatar_shape} size={40} />
            <span className={styles.texts}>
                <span className={styles.name}>{profile.display_name}</span>
                <span className={styles.username}>@{profile.username}</span>
            </span>
        </>
    ) : (
        <>
            <Skeleton shape="circle" width={40} />
            <span className={styles.texts}>
                <Skeleton shape="line" width={140} height={16} />
                <Skeleton shape="line" width={90} height={14} />
            </span>
        </>
    )

    return (
        <div className={cn(styles.row, className)} onContextMenu={onContextMenu}>
            {onClick ? (
                <button
                    type="button"
                    className={cn(styles.main, styles.clickable)}
                    aria-label={profile ? profile.display_name : 'Пользователь'}
                    onClick={onClick}
                >
                    {body}
                </button>
            ) : (
                <span className={styles.main}>{body}</span>
            )}
            {aside && (
                <span className={styles.aside}>
                    {typeof aside === 'function' ? aside(profile) : aside}
                </span>
            )}
        </div>
    )
}
