import type { QueryClient } from '@tanstack/react-query'

import { conversationKey, invalidateGroup, useGroupCard } from '@/entities/conversation'
import type { ServerEvent } from '@/shared/api'
import { notify } from '@/shared/lib'

import { findChat, findUser } from './messageNotice'

interface GroupData {
    conversation_id?: string
    user_id?: string
}

const GROUP_EVENTS = new Set([
    'conversation.updated',
    'conversation.deleted',
    'conversation.member.added',
    'conversation.member.removed',
    'conversation.join_request.created',
    'conversation.join_request.approved',
])

// события группы: перечитать карточку и списки окна группы; удалённая группа или моё исключение
// закрывают её окно (список чатов перечитывает applyConversationEvent)
export function applyGroupEvent(event: ServerEvent, queryClient: QueryClient, myId?: string) {
    if (!GROUP_EVENTS.has(event.type)) {
        return
    }
    const data = (event.data ?? {}) as GroupData
    const id = data.conversation_id
    if (!id) {
        return
    }
    const gone =
        event.type === 'conversation.deleted' ||
        (event.type === 'conversation.member.removed' && data.user_id === myId)
    if (gone) {
        const { target, close } = useGroupCard.getState()
        if (target?.kind === 'chat' && target.id === id) {
            close()
        }
        queryClient.removeQueries({ queryKey: conversationKey(id) })
        return
    }
    invalidateGroup(queryClient, id)
}

// новая заявка - админам, одобренная - тому, кто её подал
export async function notifyGroupEvent(
    event: ServerEvent,
    queryClient: QueryClient,
    myId: string | undefined,
    // что делает клик по уведомлению
    open?: { chat: (chatId: string) => void; group: (chatId: string) => void },
) {
    const data = (event.data ?? {}) as GroupData
    if (!data.conversation_id || !data.user_id) {
        return
    }
    try {
        if (event.type === 'conversation.join_request.created') {
            const [chat, user] = await Promise.all([
                findChat(queryClient, data.conversation_id),
                findUser(queryClient, data.user_id),
            ])
            notify({
                id: `join-request-${data.conversation_id}`,
                icon: 'users',
                title: `Новая заявка в «${chat.name ?? 'группу'}»`,
                description: `${user.display_name} хочет вступить`,
                // заявки рассматриваются в окне группы
                onClick: open ? () => open.group(data.conversation_id!) : undefined,
            })
        } else if (event.type === 'conversation.join_request.approved' && data.user_id === myId) {
            const chat = await findChat(queryClient, data.conversation_id)
            notify({
                icon: 'check',
                title: 'Заявка одобрена',
                description: `Вы вступили в «${chat.name ?? 'группу'}»`,
                onClick: open ? () => open.chat(data.conversation_id!) : undefined,
            })
        }
    } catch {
        // без имени группы уведомление бессмысленно - данные и так обновятся
    }
}
