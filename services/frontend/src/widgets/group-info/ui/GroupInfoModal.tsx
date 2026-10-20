import { useState } from 'react'

import { useGroupCard } from '@/entities/conversation'

import { ChatInfo } from './ChatInfo'
import { PreviewInfo } from './PreviewInfo'

// окно группы: у своей - сведения, настройки и участники; у чужой - превью со «Вступить»
export function GroupInfoModal() {
    const target = useGroupCard((state) => state.target)
    const close = useGroupCard((state) => state.close)
    // при закрытии содержимое остаётся прежним, пока играет анимация
    const [shown, setShown] = useState(target)
    if (target && target !== shown) {
        setShown(target)
    }
    // каждое открытие - с первого экрана (сведения), а не с того, на котором окно закрыли
    const open = target !== null
    const [session, setSession] = useState(0)
    const [wasOpen, setWasOpen] = useState(open)
    if (open !== wasOpen) {
        setWasOpen(open)
        if (open) {
            setSession((n) => n + 1)
        }
    }

    if (!shown) {
        return null
    }
    return shown.kind === 'chat' ? (
        <ChatInfo key={`${session}:chat:${shown.id}`} id={shown.id} open={open} onClose={close} />
    ) : (
        <PreviewInfo
            key={`${session}:preview:${shown.group.id}`}
            group={shown.group}
            open={open}
            onClose={close}
        />
    )
}
