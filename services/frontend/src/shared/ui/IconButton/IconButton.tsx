import type { ButtonHTMLAttributes } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './IconButton.module.scss'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
    icon: string
    'aria-label': string
    isActive?: boolean
}

export function IconButton({ icon, type = 'button', className, isActive, ...rest }: Props) {
    return (
        <button
            {...rest}
            type={type}
            className={cn(styles.button, isActive && styles.active, className)}
        >
            <Icon name={icon} size={22} color="inherit" />
        </button>
    )
}
