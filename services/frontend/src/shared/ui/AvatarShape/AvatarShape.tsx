import type { ComponentType, ReactNode, SVGProps } from 'react'

import styles from './AvatarShape.module.scss'

type ShapeComponent = ComponentType<SVGProps<SVGSVGElement>>

// файлы shapes/<ключ>.svg -> компонент по ключу из каталога бэкенда
const modules = import.meta.glob<ShapeComponent>('./shapes/*.svg', {
    query: '?react',
    import: 'default',
    eager: true,
})

const shapes: Record<string, ShapeComponent> = Object.fromEntries(
    Object.entries(modules).map(([path, component]) => [
        path.replace('./shapes/', '').replace('.svg', ''),
        component,
    ]),
)

export function isKnownShape(shape: string | null | undefined): shape is string {
    return !!shape && Object.hasOwn(shapes, shape)
}

type Props = {
    shape?: string | null
    color?: string | null
    className?: string
    children?: ReactNode
}

export function AvatarShape({ shape, color, className, children }: Props) {
    const known = isKnownShape(shape)
    const Shape = known ? shapes[shape]! : null

    return (
        <span className={[styles.root, className].filter(Boolean).join(' ')}>
            {Shape && (
                <Shape
                    className={styles.shape}
                    style={color ? { color } : undefined}
                    aria-hidden="true"
                    focusable="false"
                />
            )}
            <span className={styles.content}>{children}</span>
        </span>
    )
}
