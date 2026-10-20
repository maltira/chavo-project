import { useId } from 'react'
import type { ReactNode } from 'react'

import { cn } from '@/shared/lib'

import styles from './Field.module.scss'

// атрибуты, которые Field передаёт полю: связь с подписью и с текстом под полем
export interface FieldControlProps {
    id: string
    'aria-invalid'?: true
    'aria-describedby'?: string
}

interface Props {
    label: string
    hint?: string
    error?: string
    className?: string
    children: (props: FieldControlProps) => ReactNode
}

export function Field({ label, hint, error, className, children }: Props) {
    const id = useId()
    const messageId = `${id}-message`
    const message = error || hint

    return (
        <div className={cn(styles.root, className)}>
            <label htmlFor={id} className={styles.label}>
                {label}
            </label>
            {children({
                id,
                'aria-invalid': error ? true : undefined,
                'aria-describedby': message ? messageId : undefined,
            })}
            {message && (
                <p
                    id={messageId}
                    role={error ? 'alert' : undefined}
                    className={cn(styles.message, error && styles.error)}
                >
                    {message}
                </p>
            )}
        </div>
    )
}
