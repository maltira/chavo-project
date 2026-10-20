import { useEffect } from 'react'
import type { RefObject } from 'react'

// закрытие выпадающих элементов:
// - клик вне области и Esc;
// - слушатели есть только пока элемент открыт
export function useDismiss(
    open: boolean,
    ref: RefObject<HTMLElement | null>,
    onDismiss: () => void,
) {
    useEffect(() => {
        if (!open) {
            return
        }
        function onPointerDown(event: MouseEvent) {
            if (!ref.current?.contains(event.target as Node)) {
                onDismiss()
            }
        }
        function onKeyDown(event: KeyboardEvent) {
            if (event.key === 'Escape') {
                onDismiss()
            }
        }
        document.addEventListener('mousedown', onPointerDown)
        document.addEventListener('keydown', onKeyDown)
        return () => {
            document.removeEventListener('mousedown', onPointerDown)
            document.removeEventListener('keydown', onKeyDown)
        }
    }, [open, ref, onDismiss])
}
