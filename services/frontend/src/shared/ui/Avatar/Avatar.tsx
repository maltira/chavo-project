import { cn } from '@/shared/lib'

import placeholder from './avatar-placeholder.svg'
import styles from './Avatar.module.scss'

interface Props {
    // ссылка на фото; без неё показывается заглушка
    src?: string | null
    size?: number
    alt?: string
    className?: string
}

export function Avatar({ src, size = 96, alt = '', className }: Props) {
    return (
        <img
            src={src || placeholder}
            alt={alt}
            width={size}
            height={size}
            draggable={false}
            className={cn(styles.avatar, className)}
        />
    )
}
