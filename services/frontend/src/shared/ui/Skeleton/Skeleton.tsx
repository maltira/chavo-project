import type { CSSProperties } from 'react'

import { cn } from '@/shared/lib'

import styles from './Skeleton.module.scss'

interface Props {
    // круг под аватар, полоска под текст, пузырь под сообщение
    shape: 'circle' | 'line' | 'bubble'

    width?: number | string
    height?: number | string

    tone?: 'default' | 'muted'
    className?: string
    style?: CSSProperties
}

// серая мигающая заглушка на месте загружаемого содержимого
export function Skeleton({ shape, width, height, tone = 'default', className, style }: Props) {
    const size = shape === 'circle' ? (width ?? 44) : width
    return (
        <span
            aria-hidden="true"
            className={cn(
                styles.skeleton,
                styles[shape],
                tone === 'muted' && styles.muted,
                className,
            )}
            style={{
                width: size,
                height: shape === 'circle' ? size : height,
                ...style,
            }}
        />
    )
}

interface RowProps {
    compact?: boolean
    className?: string
}

// строка списка (чат, человек, группа): аватар и две строки текста
export function SkeletonRow({ compact = false, className }: RowProps) {
    return (
        <div className={cn(styles.row, compact && styles.compact, className)} aria-hidden="true">
            <Skeleton shape="circle" width={44} />
            {!compact && (
                <span className={styles.texts}>
                    <Skeleton shape="line" width="60%" />
                    <Skeleton shape="line" width="40%" />
                </span>
            )}
        </div>
    )
}
