import type { ReactNode } from 'react'

import { Icon, Loader } from '@/shared/ui'

import styles from './AuthStatus.module.scss'

interface Props {
    // loading - вращающаяся фигура, success и error - иконки
    state: 'loading' | 'success' | 'error'
    children: ReactNode
    // кнопки под текстом, например «Войти»
    actions?: ReactNode
}

const ICONS = { success: 'shape-check', error: 'shape-warn' } as const

// карточка-состояние: «Выполняем ваш запрос...», «Почта подтверждена» и т.п.
export function AuthStatus({ state, children, actions }: Props) {
    return (
        <div className={styles.root}>
            {state === 'loading' ? (
                <Loader size={64} />
            ) : (
                <Icon name={ICONS[state]} size={64} color="var(--color-text)" />
            )}
            <p className={styles.text} role={state === 'error' ? 'alert' : undefined}>
                {children}
            </p>
            {actions && <div className={styles.actions}>{actions}</div>}
        </div>
    )
}
