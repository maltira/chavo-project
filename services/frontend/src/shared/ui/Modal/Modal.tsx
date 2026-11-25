import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './Modal.module.scss'
import { useModalLayer } from './useModalLayer'

interface Props {
    open: boolean
    onClose: () => void
    // заголовок шапки (Lora); без него шапка - только слоты и крестик
    title?: string
    // по центру (как «Участники») или слева (как «Вы уверены?»)
    titleAlign?: 'center' | 'start'
    // слот слева в шапке: статус, кнопка «назад»
    headerStart?: ReactNode
    // подпись для скринридера, если заголовка нет
    'aria-label'?: string
    width?: number
    className?: string
    children: ReactNode
}

// модальное окно поверх приложения: закрывается крестиком, Esc и кликом по фону;
// фокус уходит внутрь окна и возвращается туда, откуда его открыли
export function Modal({
    open,
    onClose,
    title,
    titleAlign = 'center',
    headerStart,
    'aria-label': ariaLabel,
    width = 471,
    className,
    children,
}: Props) {
    // окно остаётся в дереве, пока играет анимация закрытия
    const [mounted, setMounted] = useState(open)
    if (open && !mounted) {
        setMounted(true)
    }
    const dialogRef = useRef<HTMLDivElement>(null)

    useModalLayer(open, onClose, dialogRef)

    if (!mounted) {
        return null
    }

    return createPortal(
        <div
            className={styles.backdrop}
            data-state={open ? 'open' : 'closed'}
            onMouseDown={(event) => {
                // клик именно по фону, а не по окну
                if (event.target === event.currentTarget) {
                    onClose()
                }
            }}
            onAnimationEnd={(event) => {
                if (!open && event.target === event.currentTarget) {
                    setMounted(false)
                }
            }}
        >
            <div
                ref={dialogRef}
                role="dialog"
                aria-modal="true"
                aria-label={title ?? ariaLabel}
                tabIndex={-1}
                className={cn(styles.dialog, className)}
                style={{ width }}
            >
                <div className={styles.header}>
                    <div className={styles.start}>
                        {titleAlign === 'start' && title ? (
                            <h2 className={styles.title}>{title}</h2>
                        ) : (
                            headerStart
                        )}
                    </div>
                    {titleAlign === 'center' && title && <h2 className={styles.title}>{title}</h2>}
                    <button
                        type="button"
                        className={styles.close}
                        aria-label="Закрыть"
                        onClick={onClose}
                    >
                        <Icon name="close" size={24} color="inherit" />
                    </button>
                </div>
                {children}
            </div>
        </div>,
        document.body,
    )
}
