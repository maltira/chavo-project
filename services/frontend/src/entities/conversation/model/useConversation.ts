import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import { getConversation, getMembers } from '../api/conversations'
import type { Member } from '../api/conversations'
import { findInLists } from './useConversations'

export const conversationKey = (id: string) => ['conversation', id] as const
export const membersKey = (id: string) => ['members', id] as const

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
