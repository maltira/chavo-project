import type { ComponentProps } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './Input.module.scss'

type RightIconProps = { rightIcon?: string; onRightIconClick?: () => void; rightIconLabel?: string }

type Props = Omit<ComponentProps<'input'>, 'className'> &
    RightIconProps & {
        leftIcon?: string
        className?: string
    }

export function Input({
    leftIcon,
    rightIcon,
    onRightIconClick,
    rightIconLabel,
    type = 'text',
    className,
    disabled,
    ...rest
}: Props) {
    return (
        <div
            className={cn(
                styles.root,
                leftIcon && styles.withLeft,
                rightIcon && styles.withRight,
                disabled && styles.disabled,
                className,
            )}
        >
            {leftIcon && (
                <span className={cn(styles.icon, styles.left)}>
                    <Icon name={leftIcon} size={24} color="inherit" />
                </span>
            )}

            <input {...rest} type={type} disabled={disabled} className={styles.input} />

            {rightIcon && (
                <button
                    type="button"
                    className={cn(styles.icon, styles.right)}
                    aria-label={rightIconLabel}
                    disabled={disabled}
                    onClick={onRightIconClick}
                >
                    <Icon name={rightIcon} size={22} color="inherit" />
                </button>
            )}
        </div>
    )
}
