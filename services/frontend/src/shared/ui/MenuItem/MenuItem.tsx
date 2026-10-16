import type { ReactNode } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './MenuItem.module.scss'

interface Props {
    icon: string
    children: ReactNode
    danger?: boolean
    trailing?: ReactNode // элемент справа от подписи
    disabled?: boolean
    onClick?: () => void
    className?: string
}

export function MenuItem({
    icon,
    children,
    danger = false,
    trailing,
    disabled,
    onClick,
    className,
}: Props) {
    const content = (
        <>
            <span className={styles.icon}>
                <Icon name={icon} size={22} color="inherit" />
            </span>
            <span className={styles.label}>{children}</span>
            {trailing}
        </>
    )
    const rootClass = cn(styles.root, danger && styles.danger, className)

    if (onClick) {
        return (
            <button
                type="button"
                role="menuitem"
                className={cn(rootClass, styles.interactive)}
                disabled={disabled}
                onClick={onClick}
            >
                {content}
            </button>
        )
    }

    // без onClick, но с элементом управления справа (Switch): клик по строке переключает его
    if (trailing) {
        return <label className={cn(rootClass, styles.interactive)}>{content}</label>
    }

    return (
        <div role="menuitem" className={rootClass}>
            {content}
        </div>
    )
}
