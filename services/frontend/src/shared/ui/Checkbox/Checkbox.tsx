import type { ComponentProps, ReactNode } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './Checkbox.module.scss'

type Props = Omit<ComponentProps<'input'>, 'type'> & {
    // подпись справа; нажатие по ней тоже переключает
    children: ReactNode
}

// флажок с подписью: настоящий input под квадратом, поэтому работают Tab, пробел и скринридер
export function Checkbox({ className, disabled, children, ...rest }: Props) {
    return (
        <label className={cn(styles.root, disabled && styles.disabled, className)}>
            <span className={styles.control}>
                <input {...rest} type="checkbox" disabled={disabled} className={styles.input} />
                <span className={styles.box}>
                    <Icon name="check" size={14} color="currentColor" className={styles.mark} />
                </span>
            </span>
            <span className={styles.label}>{children}</span>
        </label>
    )
}
