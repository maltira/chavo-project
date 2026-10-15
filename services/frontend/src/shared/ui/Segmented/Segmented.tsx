import { cn } from '@/shared/lib'

import styles from './Segmented.module.scss'

export interface SegmentedOption<T extends string> {
    value: T
    label: string
}

interface Props<T extends string> {
    options: SegmentedOption<T>[]
    value: T
    onChange: (value: T) => void
    'aria-label': string
    className?: string
}

// переключатель из нескольких вариантов (например, «Открытый / Закрытый»)
export function Segmented<T extends string>({
    options,
    value,
    onChange,
    'aria-label': ariaLabel,
    className,
}: Props<T>) {
    return (
        <div role="radiogroup" aria-label={ariaLabel} className={cn(styles.root, className)}>
            {options.map((option) => (
                <button
                    key={option.value}
                    type="button"
                    role="radio"
                    aria-checked={option.value === value}
                    className={styles.option}
                    onClick={() => onChange(option.value)}
                >
                    {option.label}
                </button>
            ))}
        </div>
    )
}
