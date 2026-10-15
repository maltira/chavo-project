import type { ReactNode } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './InfoList.module.scss'

export interface InfoItem {
    icon: string
    label: string
    value: ReactNode
}

interface Props {
    items: InfoItem[]
    className?: string
}

// карточка сведений: иконка в плитке, подпись и значение (профиль, группа)
export function InfoList({ items, className }: Props) {
    return (
        <dl className={cn(styles.root, className)}>
            {items.map((item) => (
                <div key={item.label} className={styles.row}>
                    <span className={styles.icon}>
                        <Icon name={item.icon} size={22} color="inherit" />
                    </span>
                    <div className={styles.texts}>
                        <dt className={styles.label}>{item.label}</dt>
                        <dd className={styles.value}>{item.value}</dd>
                    </div>
                </div>
            ))}
        </dl>
    )
}
