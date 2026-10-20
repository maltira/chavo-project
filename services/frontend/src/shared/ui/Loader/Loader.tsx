import { useEffect, useState } from 'react'
import type { ComponentType, SVGProps } from 'react'

import { cn } from '@/shared/lib'

import styles from './Loader.module.scss'

type FrameComponent = ComponentType<SVGProps<SVGSVGElement>>

// файлы frames/<номер>.svg - состояния фигуры по порядку
const modules = import.meta.glob<FrameComponent>('./frames/*.svg', {
    query: '?react',
    import: 'default',
    eager: true,
})

const frames: FrameComponent[] = Object.entries(modules)
    .map(([path, component]) => ({
        order: Number(path.replace('./frames/', '').replace('.svg', '')),
        component,
    }))
    .sort((a, b) => a.order - b.order)
    .map((frame) => frame.component)

interface Props {
    size?: number
    frameMs?: number // время показа одного состояния, мс
    color?: string // цвет фигуры, по умолчанию цвет текста темы
    showContainer?: boolean // круг цвета accent позади фигуры, size - диаметр круга
}

// фигура по кругу меняет состояния: кадры лежат стопкой, активный проявляется, прежний гаснет
export function Loader({
    size = 48,
    frameMs = 800,
    color = 'var(--color-text)',
    showContainer = false,
}: Props) {
    const [active, setActive] = useState(0)

    useEffect(() => {
        if (frames.length < 2) {
            return
        }
        const timer = setInterval(() => setActive((i) => (i + 1) % frames.length), frameMs)
        return () => clearInterval(timer)
    }, [frameMs])

    return (
        <span
            role="status"
            className={cn(styles.root, showContainer && styles.withContainer)}
            style={{ width: size, height: size, color }}
        >
            <span className={styles.shapes}>
                {frames.map((Frame, index) => (
                    <Frame
                        key={index}
                        aria-hidden="true"
                        focusable="false"
                        data-active={index === active ? 'true' : undefined}
                        className={styles.frame}
                    />
                ))}
            </span>
        </span>
    )
}
