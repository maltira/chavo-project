import { cn } from '@/shared/lib'
import styles from './Tag.module.scss'
import type { ReactNode } from 'react'

interface Props {
    variant?: 'default' | 'muted'
    children: ReactNode
}

export function Tag({ variant = 'default', children }: Props) {
    return <span className={cn(styles.tag, styles[variant])}>{children}</span>
}
