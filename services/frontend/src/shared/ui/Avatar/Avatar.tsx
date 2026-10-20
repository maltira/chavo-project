import { useState } from 'react'

import { cn } from '@/shared/lib'

import placeholder from './avatar-placeholder.svg'
import styles from './Avatar.module.scss'

interface Props {
    // ссылка на фото; без неё (или если фото не загрузилось) показывается заглушка
    src?: string | null
    size?: number
    alt?: string
    className?: string
}

export function Avatar({ src, size = 96, alt = '', className }: Props) {
    // фото, которое не загрузилось: вместо битой картинки - заглушка, пока не придёт другая ссылка
    const [failed, setFailed] = useState<string | null>(null)
    const shown = src && src !== failed ? src : placeholder

    return (
        <img
            src={shown}
            alt={alt}
            width={size}
            height={size}
            loading="lazy"
            decoding="async"
            draggable={false}
            className={cn(styles.avatar, className)}
            onError={() => src && setFailed(src)}
        />
    )
}
