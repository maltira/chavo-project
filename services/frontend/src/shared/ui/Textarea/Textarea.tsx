import type { ComponentProps } from 'react'

import { cn } from '@/shared/lib'

import styles from './Textarea.module.scss'

export function Textarea({ className, ...rest }: ComponentProps<'textarea'>) {
    return <textarea {...rest} className={cn(styles.textarea, className)} />
}
