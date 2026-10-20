import { Skeleton } from '@/shared/ui'

import styles from './ChatSkeleton.module.scss'

// пузыри-заглушки снизу вверх: свои справа, чужие слева, разной ширины - как живая переписка
const BUBBLES: { mine: boolean; width: string }[] = [
    { mine: true, width: '38%' },
    { mine: false, width: '52%' },
    { mine: false, width: '30%' },
    { mine: true, width: '46%' },
    { mine: false, width: '60%' },
    { mine: true, width: '26%' },
    { mine: false, width: '42%' },
]

// история, пока загружается
export function MessagesSkeleton() {
    return (
        <div className={styles.messages} aria-busy="true" aria-label="Загрузка сообщений">
            {BUBBLES.map((bubble, index) => (
                <Skeleton
                    key={index}
                    shape="bubble"
                    width={bubble.width}
                    tone={bubble.mine ? 'muted' : 'default'}
                    className={bubble.mine ? styles.mine : undefined}
                />
            ))}
        </div>
    )
}

// шапка чата, пока карточка чата не загрузилась (фон шапки - bg-tertiary, заглушки светлее)
export function HeaderSkeleton() {
    return (
        <div className={styles.header} aria-hidden="true">
            <Skeleton shape="circle" width={44} tone="muted" />
            <span className={styles.texts}>
                <Skeleton shape="line" width="30%" tone="muted" />
                <Skeleton shape="line" width="20%" tone="muted" />
            </span>
        </div>
    )
}
