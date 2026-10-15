import { useEffect, useRef } from 'react'

import { markRead } from '@/entities/message'

// пауза после последнего увиденного сообщения: при чтении по одному сообщению запросы сливаются в один
const DEBOUNCE_MS = 400

// курсор прочтения двигается на самое свежее сообщение, которое пользователь действительно видел
// (его сообщает история по мере прокрутки), и только пока вкладка на виду;
// отправка откладывается и объединяется, а при уходе из чата накопленное уходит сразу;
// счётчик в списке пересчитает сервер: своё событие message.read перечитывает список
export function useMarkRead(conversationId: string, seenId: string | undefined) {
    // что уже ушло на сервер и что ждёт отправки
    const sentRef = useRef<string | null>(null)
    const pendingRef = useRef<string | null>(null)
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    // отправляет ожидающее значение немедленно
    function flush() {
        if (timerRef.current) {
            clearTimeout(timerRef.current)
            timerRef.current = null
        }
        const id = pendingRef.current
        if (!id || id === sentRef.current || document.visibilityState !== 'visible') {
            return
        }
        pendingRef.current = null
        sentRef.current = id
        // ошибка не мешает: курсор подвинется со следующим увиденным сообщением
        markRead(conversationId, id).catch(() => {
            sentRef.current = null
        })
    }

    useEffect(() => {
        if (!seenId || seenId.startsWith('temp-') || seenId === sentRef.current) {
            return
        }
        pendingRef.current = seenId
        if (timerRef.current) {
            clearTimeout(timerRef.current)
        }
        timerRef.current = setTimeout(flush, DEBOUNCE_MS)
        // flush читает только ref'ы и conversationId, поэтому перезапуск эффекта по нему не нужен
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [seenId])

    // пока вкладка скрыта, ничего не отмечается; вернулись - накопленное уходит с обычной паузой
    useEffect(() => {
        const onVisibility = () => {
            if (document.visibilityState === 'hidden') {
                if (timerRef.current) {
                    clearTimeout(timerRef.current)
                    timerRef.current = null
                }
            } else if (pendingRef.current && !timerRef.current) {
                timerRef.current = setTimeout(flush, DEBOUNCE_MS)
            }
        }
        document.addEventListener('visibilitychange', onVisibility)
        // уход из чата: накопленное прочтение не теряется
        return () => {
            document.removeEventListener('visibilitychange', onVisibility)
            flush()
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [conversationId])
}
