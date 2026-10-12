import type { ComponentType, SVGProps } from 'react'

type IconComponent = ComponentType<SVGProps<SVGSVGElement>>

// файлы icons/<имя>.svg -> компонент по имени файла
const modules = import.meta.glob<IconComponent>('./icons/*.svg', {
    query: '?react',
    import: 'default',
    eager: true,
})

const icons: Record<string, IconComponent> = Object.fromEntries(
    Object.entries(modules).map(([path, component]) => [
        path.replace('./icons/', '').replace('.svg', ''),
        component,
    ]),
)

export function isKnownIcon(name: string | null | undefined): name is string {
    return !!name && Object.hasOwn(icons, name)
}

type Props = Omit<SVGProps<SVGSVGElement>, 'ref' | 'color'> & {
    name: string
    size?: number
    color?: string
}

export function Icon({ name, size = 24, color = 'var(--color-text)', style, ...rest }: Props) {
    if (!isKnownIcon(name)) {
        return null
    }

    const Svg = icons[name]!
    return (
        <Svg
            width={size}
            height={size}
            {...rest}
            aria-hidden="true"
            focusable="false"
            style={{ ...style, color, pointerEvents: 'none', userSelect: 'none' }}
        />
    )
}
