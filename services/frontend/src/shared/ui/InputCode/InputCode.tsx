import type { ChangeEvent, ComponentProps, FocusEvent } from 'react'

import { cn } from '@/shared/lib'

import styles from './InputCode.module.scss'

type Props = Omit<ComponentProps<'input'>, 'type' | 'inputMode' | 'maxLength'> & {
    'aria-label': string
}

export function InputCode({ placeholder = '_', className, onChange, onFocus, ...rest }: Props) {
    // оставляем только последнюю введённую цифру: новая цифра заменяет старую
    function handleChange(event: ChangeEvent<HTMLInputElement>) {
        event.target.value = event.target.value.replace(/\D/g, '').slice(-1)
        onChange?.(event)
    }

    // при фокусе цифра выделяется, чтобы следующий ввод её заменил
    function handleFocus(event: FocusEvent<HTMLInputElement>) {
        event.currentTarget.select()
        onFocus?.(event)
    }

    return (
        <input
            {...rest}
            type="text"
            inputMode="numeric"
            autoComplete="off"
            placeholder={placeholder}
            className={cn(styles.input, className)}
            onChange={handleChange}
            onFocus={handleFocus}
        />
    )
}
