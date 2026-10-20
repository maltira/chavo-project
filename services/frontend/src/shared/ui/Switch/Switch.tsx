import type { ComponentProps } from 'react'

import { cn } from '@/shared/lib'

import styles from './Switch.module.scss'

type Props = Omit<ComponentProps<'input'>, 'type' | 'role'> & {
    'aria-label': string
}

export function Switch({ className, disabled, ...rest }: Props) {
    return (
        <span className={cn(styles.root, disabled && styles.disabled, className)}>
            <input
                {...rest}
                type="checkbox"
                role="switch"
                disabled={disabled}
                className={styles.input}
            />
            <span className={styles.track}>
                <span className={styles.thumb} />
            </span>
        </span>
    )
}
