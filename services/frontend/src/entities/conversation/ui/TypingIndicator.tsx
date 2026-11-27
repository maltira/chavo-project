import { cn } from '@/shared/lib'

import styles from './TypingIndicator.module.scss'

interface Props {
    label: string
    className?: string
}

// индикатор набора с цикличными точками . -> .. -> ...
export function TypingIndicator({ label, className }: Props) {
    return (
        <span className={cn(styles.root, className)} aria-label={`${label}…`}>
            <span className={styles.label}>{label}</span>
            <span className={styles.dots} aria-hidden="true">
                <span>.</span>
                <span className={styles.dot2}>.</span>
                <span className={styles.dot3}>.</span>
            </span>
        </span>
    )
}
