import type { ReactNode } from 'react'

import { cn } from '@/shared/lib'

import styles from './AppearRow.module.scss'

interface Props {
    // пришло при открытом чате: строка раскрывается по высоте, а не появляется сразу
    appear: boolean
    // продолжение серии: отступ сверху меньше обычного (у строк с анимацией он задаётся здесь)
    continued: boolean
    children: ReactNode
}

// обёртка строки истории: высота растёт от нуля вместе с появлением сообщения,
// поэтому старые сообщения уезжают вверх плавно, а не прыгают на высоту нового
export function AppearRow({ appear, continued, children }: Props) {
    if (!appear) {
        return <>{children}</>
    }
    return (
        <div className={cn(styles.grow, continued && styles.continued)}>
            <div className={styles.inner}>{children}</div>
        </div>
    )
}
