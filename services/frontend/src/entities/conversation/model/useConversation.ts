import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import { getConversation, getMembers, listUnreadMentions } from '../api/conversations'
import type { Member } from '../api/conversations'
import { findInLists } from './useConversations'

export const conversationKey = (id: string) => ['conversation', id] as const
export const membersKey = (id: string) => ['members', id] as const
export const unreadMentionsKey = (id: string) => ['unread-mentions', id] as const

// карточка чата; пока загружается, берётся строка из списка - окно открывается без ожидания
export function useConversation(id: string, enabled = true) {
    const queryClient = useQueryClient()
    return useQuery({
        queryKey: conversationKey(id),
        queryFn: () => getConversation(id),
        placeholderData: () => findInLists(queryClient, id),
        enabled,
    })
}

// участники с курсорами прочтения: от них зависят галочки у своих сообщений
export function useMembers(id: string | null | undefined) {
    return useQuery({
        queryKey: membersKey(id ?? ''),
        queryFn: () => getMembers(id!),
        enabled: !!id,
    })
}

// непрочитанные упоминания меня в чате (для кнопки @)
export function useUnreadMentions(id: string | null | undefined, enabled = true) {
    return useQuery({
        queryKey: unreadMentionsKey(id ?? ''),
        queryFn: () => listUnreadMentions(id!),
        enabled: !!id && enabled,
    })
}

// событие message.read двигает курсор участника вперёд
export function moveReadCursor(
    queryClient: QueryClient,
    conversationId: string,
    readerId: string,
    messageId: string,
) {
    queryClient.setQueryData<Member[]>(membersKey(conversationId), (members) =>
        members?.map((m) =>
            m.user_id === readerId ? { ...m, last_read_message_id: messageId } : m,
        ),
    )
}
