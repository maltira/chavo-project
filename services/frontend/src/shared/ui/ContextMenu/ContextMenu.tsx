import { useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { useDismiss } from '@/shared/lib'

import styles from './ContextMenu.module.scss'

export interface MenuPosition {
    x: number
    y: number
    // end - x это правый край меню (меню под кнопкой у правого края экрана)
    align?: 'start' | 'end'
}

interface Props {
    // null - меню закрыто
    position: MenuPosition | null
    onClose: () => void
    'aria-label': string
    children: ReactNode
}

// отступ меню от края окна
const EDGE = 8

// всплывающее меню у курсора (правая кнопка) или под кнопкой «⋮»;
// у края окна сдвигается внутрь, чтобы не обрезаться
export function ContextMenu({ position, onClose, 'aria-label': ariaLabel, children }: Props) {
    const ref = useRef<HTMLDivElement>(null)
    const [placed, setPlaced] = useState<MenuPosition | null>(null)
    const open = position !== null

    useDismiss(open, ref, onClose)

    // размер меню известен только после отрисовки: подвинуть до того, как браузер его покажет
    useLayoutEffect(() => {
        const node = ref.current
        if (!position || !node) {
            setPlaced(null)
            return
        }
        const width = node.offsetWidth
        const height = node.offsetHeight
        const left = position.align === 'end' ? position.x - width : position.x
        setPlaced({
            x: Math.max(EDGE, Math.min(left, window.innerWidth - width - EDGE)),
            y: Math.max(EDGE, Math.min(position.y, window.innerHeight - height - EDGE)),
        })
    }, [position])

    if (!position) {
        return null
    }

    const at = placed ?? position
    return createPortal(
        <div
            ref={ref}
            role="menu"
            aria-label={ariaLabel}
            className={styles.menu}
            style={{
                left: at.x,
                top: at.y,
                transformOrigin: position.align === 'end' ? 'top right' : 'top left',
            }}
            // пункт выбран - меню закрывается
            onClick={(event) => {
                if ((event.target as HTMLElement).closest('[role="menuitem"]')) {
                    onClose()
                }
            }}
            onContextMenu={(event) => event.preventDefault()}
        >
            {children}
        </div>,
        document.body,
    )
}
