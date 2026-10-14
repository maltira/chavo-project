import { useRef } from 'react'
import type { PointerEvent, RefObject } from 'react'

import { DEFAULT_WIDTH } from '../model/sidebarWidth'
import styles from './SidebarResizer.module.scss'

interface Props {
    // сайдбар, у которого меняется ширина: отсюда берётся левый край для расчёта
    target: RefObject<HTMLElement | null>
    onPreview: (raw: number) => void
    onCommit: (raw: number) => void
    onDragChange: (dragging: boolean) => void
}

// узкая полоса по правому краю сайдбара: тянется мышью или пальцем, двойной щелчок сбрасывает ширину
export function SidebarResizer({ target, onPreview, onCommit, onDragChange }: Props) {
    const lastRaw = useRef(0)
    const dragging = useRef(false)

    function rawFrom(event: PointerEvent) {
        const left = target.current?.getBoundingClientRect().left ?? 0
        return event.clientX - left
    }

    function onPointerDown(event: PointerEvent<HTMLDivElement>) {
        if (event.button !== 0) {
            return
        }
        dragging.current = true
        lastRaw.current = rawFrom(event)
        event.currentTarget.setPointerCapture?.(event.pointerId)
        onDragChange(true)
        event.preventDefault()
    }

    function onPointerMove(event: PointerEvent<HTMLDivElement>) {
        if (!dragging.current) {
            return
        }
        lastRaw.current = rawFrom(event)
        onPreview(lastRaw.current)
    }

    function finish(event: PointerEvent<HTMLDivElement>) {
        if (!dragging.current) {
            return
        }
        dragging.current = false
        event.currentTarget.releasePointerCapture?.(event.pointerId)
        onDragChange(false)
        onCommit(lastRaw.current)
    }

    return (
        <div
            role="separator"
            aria-orientation="vertical"
            aria-label="Ширина списка чатов"
            className={styles.resizer}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={finish}
            onPointerCancel={finish}
            // двойной щелчок возвращает ширину по умолчанию
            onDoubleClick={() => onCommit(DEFAULT_WIDTH)}
        />
    )
}
