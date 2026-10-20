import type { ButtonHTMLAttributes } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './IconButton.module.scss'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
    icon: string
    'aria-label': string
    isActive?: boolean
    size?: number
}

export function IconButton({
    icon,
    type = 'button',
    className,
    isActive,
    size,
    style,
    ...rest
}: Props) {
    return (
        <button
            {...rest}
            type={type}
            className={cn(styles.button, isActive && styles.active, className)}
            style={size ? { ...style, width: size, height: size } : style}
        >
            <Icon name={icon} size={22} color="inherit" />
        </button>
    )
}
