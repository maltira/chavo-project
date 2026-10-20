import { useRef, useState } from 'react'

import { cn, useDismiss } from '@/shared/lib'

import { IconButton } from '../IconButton'
import { Tag } from '../Tag'
import styles from './LanguageMenu.module.scss'

export interface LanguageOption {
    value: string
    tag: string
    label: string
}

interface Props {
    value: string
    options: LanguageOption[]
    onChange: (value: string) => void
}

export function LanguageMenu({ value, options, onChange }: Props) {
    const [open, setOpen] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)

    useDismiss(open, rootRef, () => setOpen(false))

    function select(next: string) {
        onChange(next)
        setOpen(false)
    }

    return (
        <div ref={rootRef} className={styles.root}>
            <IconButton
                size={40}
                icon="language"
                aria-label="Выбрать язык"
                aria-haspopup="menu"
                aria-expanded={open}
                isActive={open}
                onClick={() => setOpen((v) => !v)}
            />
            {open && (
                <div role="menu" aria-label="Язык" className={styles.menu}>
                    {options.map((option) => (
                        <button
                            key={option.value}
                            type="button"
                            role="menuitemradio"
                            aria-checked={option.value === value}
                            aria-label={option.label}
                            className={cn(styles.item, option.value === value && styles.selected)}
                            onClick={() => select(option.value)}
                        >
                            <Tag variant="muted">{option.tag}</Tag>
                            <span className={styles.label}>{option.label}</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}
