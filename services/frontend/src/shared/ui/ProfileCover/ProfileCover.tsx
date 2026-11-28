import type { ComponentType, CSSProperties, SVGProps } from 'react'

import { cn } from '@/shared/lib'

import styles from './ProfileCover.module.scss'

type PatternComponent = ComponentType<SVGProps<SVGSVGElement>>

// файлы patterns/<ключ>.svg -> компонент по ключу из каталога бэкенда
const modules = import.meta.glob<PatternComponent>('./patterns/*.svg', {
    query: '?react',
    import: 'default',
    eager: true,
})

const patterns: Record<string, PatternComponent> = Object.fromEntries(
    Object.entries(modules).map(([path, component]) => [
        path.replace('./patterns/', '').replace('.svg', ''),
        component,
    ]),
)

export function isKnownCoverPattern(pattern: string | null | undefined): pattern is string {
    return !!pattern && Object.hasOwn(patterns, pattern)
}

// симметричная орбитальная россыпь символов вокруг аватара (центр свободен под аватар)
const ORBIT_POINTS: readonly {
    x: number
    y: number
    size: number
    rotate: number
    opacity: number
}[] = [
    // верхняя дуга
    { x: 18, y: 14, size: 15, rotate: -18, opacity: 0.34 },
    { x: 35, y: 9, size: 13, rotate: 12, opacity: 0.28 },
    { x: 65, y: 9, size: 13, rotate: -12, opacity: 0.28 },
    { x: 82, y: 14, size: 15, rotate: 18, opacity: 0.34 },
    // внутреннее кольцо по бокам от аватара
    { x: 26, y: 31, size: 19, rotate: -14, opacity: 0.44 },
    { x: 74, y: 31, size: 19, rotate: 14, opacity: 0.44 },
    { x: 11, y: 37, size: 14, rotate: 22, opacity: 0.26 },
    { x: 89, y: 37, size: 14, rotate: -22, opacity: 0.26 },
    // нижнее кольцо вокруг аватара и над текстом
    { x: 22, y: 55, size: 17, rotate: 16, opacity: 0.36 },
    { x: 78, y: 55, size: 17, rotate: -16, opacity: 0.36 },
    { x: 8, y: 62, size: 12, rotate: -24, opacity: 0.2 },
    { x: 92, y: 62, size: 12, rotate: 24, opacity: 0.2 },
    // мягкие акценты ниже по бокам
    { x: 33, y: 69, size: 13, rotate: -10, opacity: 0.24 },
    { x: 67, y: 69, size: 13, rotate: 10, opacity: 0.24 },
]

interface PatternIconProps {
    pattern: string
    size?: number
    className?: string
}

// одиночный символ паттерна для сетки выбора в редакторе
export function CoverPatternIcon({ pattern, size = 20, className }: PatternIconProps) {
    const Pattern = isKnownCoverPattern(pattern) ? patterns[pattern]! : null
    if (!Pattern) {
        return null
    }
    return (
        <Pattern
            width={size}
            height={size}
            className={className}
            aria-hidden="true"
            focusable="false"
        />
    )
}

interface Props {
    color: string
    pattern?: string | null
    // компактный режим для превью в редакторе «Оформление»
    compact?: boolean
    // плавное раскрытие из центра при открытии карточки (по умолчанию включено вне превью)
    animate?: boolean
    className?: string
}

// шапка-ореол: мягкий радиальный градиент цвета, переходящий в фон карточки, и орбитальный паттерн
export function ProfileCover({
    color,
    pattern,
    compact = false,
    animate = !compact,
    className,
}: Props) {
    const known = isKnownCoverPattern(pattern)
    const Pattern = known ? patterns[pattern]! : null

    return (
        <div
            className={cn(
                styles.root,
                compact && styles.compact,
                animate && styles.animated,
                className,
            )}
            style={{ '--cover-color': color } as CSSProperties}
            data-cover-color={color}
            data-cover-pattern={known ? pattern : ''}
            aria-hidden="true"
        >
            <div className={styles.gradient} />
            {Pattern && (
                <div className={styles.pattern}>
                    {ORBIT_POINTS.map((pt, index) => (
                        <Pattern
                            key={index}
                            className={styles.symbol}
                            style={{
                                left: `${pt.x}%`,
                                top: `${pt.y}%`,
                                width: compact ? Math.round(pt.size * 0.85) : pt.size,
                                height: compact ? Math.round(pt.size * 0.85) : pt.size,
                                opacity: pt.opacity,
                                transform: `translate(-50%, -50%) rotate(${pt.rotate}deg)`,
                            }}
                            aria-hidden="true"
                            focusable="false"
                        />
                    ))}
                </div>
            )}
            <div className={styles.fade} />
        </div>
    )
}
