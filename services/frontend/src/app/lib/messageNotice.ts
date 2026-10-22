import type { QueryClient } from '@tanstack/react-query'

import { conversationKey, findInLists, getConversation, isMuted } from '@/entities/conversation'
import { getUser, userKey } from '@/entities/user'
import type { Profile } from '@/entities/user'
import type { ServerEvent } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

interface CreatedData {
    conversation_id?: string
    sender_id?: string
    content?: string | null
    kind?: string
}

// чат из кеша (списки или карточка), иначе с сервера
export async function findChat(queryClient: QueryClient, id: string) {
    return (
        findInLists(queryClient, id) ??
        queryClient.fetchQuery({
            queryKey: conversationKey(id),
            queryFn: () => getConversation(id),
        })
    )
}

export function findUser(queryClient: QueryClient, id: string) {
    return queryClient.fetchQuery<Profile>({
        queryKey: userKey(id),
        queryFn: () => getUser(id),
        // свежий профиль уже есть в кеше - запроса не будет
        staleTime: 5 * 60_000,
    })
}

// уведомление о новом сообщении: заголовок - собеседник или группа, текст - сообщение;
// не показываем свои сообщения и сообщения чата, который сейчас открыт и виден
export async function notifyNewMessage(
    event: ServerEvent,
    queryClient: QueryClient,
    myId: string | undefined,
    // что делает клик по уведомлению
    open?: { chat: (chatId: string) => void },
) {
    if (event.type !== 'message.created') {
        return
    }
    const data = (event.data ?? {}) as CreatedData
    const { conversation_id: chatId, sender_id: senderId } = data
    // о вступлениях и выходах не уведомляем: они видны в истории
    if (!chatId || !senderId || senderId === myId || data.kind === 'system') {
        return
    }
    const watching =
        document.visibilityState === 'visible' && window.location.pathname === chatPath(chatId)
    if (watching) {
        return
    }

    try {
        const [chat, sender] = await Promise.all([
            findChat(queryClient, chatId),
            findUser(queryClient, senderId),
        ])
        // чат без звука - без всплывашки, непрочитанные всё равно считаются
        if (isMuted(chat)) {
            return
        }
        const isGroup = chat.conversation_type === 'group'
        const text = data.content ?? ''
        notify({
            // новое сообщение того же чата заменяет прежнее уведомление, а не копит их
            id: `message-${chatId}`,
            icon: 'shape-message',
            title: isGroup ? (chat.name ?? 'Группа') : sender.display_name,
            // в группе видно, кто написал
            description: isGroup ? `${sender.display_name}: ${text}` : text,
            onClick: open ? () => open.chat(chatId) : undefined,
        })
    } catch {
        // не узнали имя (сеть, нет доступа) - без уведомления, сообщение всё равно в списке чатов
    }
}
