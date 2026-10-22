import { useEffect } from 'react'
import type { RefObject } from 'react'

// открытые окна поверх приложения, верхнее - последнее: Esc закрывает только его
// (подтверждение поверх профиля, просмотр фото поверх окна)
const stack: symbol[] = []

// слой окна: пока open, окно в стеке; фокус уходит внутрь и возвращается туда, откуда его открыли
export function useModalLayer(
    open: boolean,
    onClose: () => void,
    focusRef: RefObject<HTMLElement | null>,
) {
    useEffect(() => {
        if (!open) {
            return
        }
        const previous = document.activeElement as HTMLElement | null
        const id = Symbol('modal')
        stack.push(id)
        focusRef.current?.focus()

        function onKeyDown(event: KeyboardEvent) {
            if (event.key === 'Escape' && stack.at(-1) === id) {
                onClose()
            }
        }
        document.addEventListener('keydown', onKeyDown)
        return () => {
            document.removeEventListener('keydown', onKeyDown)
            stack.splice(stack.indexOf(id), 1)
            previous?.focus?.()
        }
    }, [open, onClose, focusRef])
}
