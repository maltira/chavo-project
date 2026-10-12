import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './Button.module.scss'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: 'primary' | 'secondary'
    icon?: string
    children?: ReactNode
}

export function Button({
    variant = 'primary',
    type = 'button',
    icon,
    className,
    children,
    ...rest
}: Props) {
    return (
        <button
            {...rest}
            type={type}
            className={cn(styles.button, styles[`button--${variant}`], className)}
        >
            {icon && <Icon name={icon} size={22} color="inherit" />}
            {children}
        </button>
    )
}
