import type { AvatarShape } from '@/entities/user'
import { UserAvatar } from '@/entities/user'
import { plural } from '@/shared/lib'

import styles from './GroupInfo.module.scss'

interface Props {
    name: string
    avatarUrl: string | null
    shape: AvatarShape | null
    membersCount: number
    // нажатие по фото открывает его на весь экран (только если фото есть)
    onOpenPhoto?: () => void
}

// аватар 96 с фигурой, название и число участников
export function GroupHead({ name, avatarUrl, shape, membersCount, onOpenPhoto }: Props) {
    const avatar = <UserAvatar src={avatarUrl} shape={shape} size={96} />
    return (
        <div className={styles.head}>
            {avatarUrl && onOpenPhoto ? (
                <button
                    type="button"
                    className={styles.avatarButton}
                    aria-label="Открыть фото группы"
                    onClick={onOpenPhoto}
                >
                    {avatar}
                </button>
            ) : (
                avatar
            )}
            <div className={styles.names}>
                <h2 className={styles.name}>{name}</h2>
                <span className={styles.muted}>
                    {membersCount} {plural(membersCount, ['участник', 'участника', 'участников'])}
                </span>
            </div>
        </div>
    )
}

export function statusProps(status: {
    icon: string
    text: string
    background: string
    foreground: string
    expires_at: string | null
}) {
    return {
        icon: status.icon,
        text: status.text,
        background: status.background,
        foreground: status.foreground,
        expiresAt: status.expires_at,
    }
}
