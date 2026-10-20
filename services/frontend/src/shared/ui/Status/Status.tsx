import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './Status.module.scss'

interface Props {
    icon: string
    text: string
    background: string
    foreground: string
    expiresAt?: string | null
    className?: string
}

// плашка статуса пользователя или группы
export function Status({ icon, text, background, foreground, expiresAt, className }: Props) {
    if (expiresAt && new Date(expiresAt) <= new Date()) {
        return null
    }

    return (
        <div
            className={cn(styles.root, className)}
            style={{ backgroundColor: background, color: foreground }}
        >
            <Icon name={`shape-${icon}`} size={24} color="inherit" />
            <span className={styles.text}>{text}</span>
        </div>
    )
}
