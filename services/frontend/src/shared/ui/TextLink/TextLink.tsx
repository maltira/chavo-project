import type { ComponentPropsWithoutRef, ElementType, ReactNode } from 'react'

import { cn } from '@/shared/lib'

import styles from './TextLink.module.scss'

type Props<T extends ElementType> = {
    as?: T
    className?: string
    children: ReactNode
} & Omit<ComponentPropsWithoutRef<T>, 'as' | 'className' | 'children'>

export function TextLink<T extends ElementType = 'a'>({
    as,
    className,
    children,
    ...rest
}: Props<T>) {
    const Component: ElementType = as ?? 'a'
    const buttonProps = Component === 'button' ? { type: 'button' } : {}

    return (
        <Component {...buttonProps} {...rest} className={cn(styles.link, className)}>
            {children}
        </Component>
    )
}
