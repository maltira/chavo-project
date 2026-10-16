import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './ColorPicker.module.scss'

interface Props {
    value: string // #RRGGBB
    onChange: (color: string) => void
    palette: string[]
    'aria-label': string
    className?: string
}

const same = (a: string, b: string) => a.toUpperCase() === b.toUpperCase()

export function ColorPicker({
    value,
    onChange,
    palette,
    'aria-label': ariaLabel,
    className,
}: Props) {
    const custom = !palette.some((color) => same(color, value))

    return (
        <div role="radiogroup" aria-label={ariaLabel} className={cn(styles.root, className)}>
            {palette.map((color) => (
                <button
                    key={color}
                    type="button"
                    role="radio"
                    aria-checked={same(color, value)}
                    aria-label={color}
                    className={styles.swatch}
                    style={{ backgroundColor: color }}
                    onClick={() => onChange(color.toUpperCase())}
                />
            ))}
            <label
                className={cn(styles.swatch, styles.custom)}
                aria-checked={custom}
                style={custom ? { backgroundColor: value } : undefined}
                title="Свой цвет"
            >
                {!custom && <Icon name="pallete" size={16} color="inherit" />}
                <input
                    type="color"
                    aria-label="Свой цвет"
                    className={styles.input}
                    value={value.toLowerCase()}
                    onChange={(event) => onChange(event.target.value.toUpperCase())}
                />
            </label>
        </div>
    )
}
