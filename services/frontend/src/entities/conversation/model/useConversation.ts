import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import { getConversation, getMembers } from '../api/conversations'
import type { Member } from '../api/conversations'
import type { ConversationSummary } from './types'
import { conversationsKey } from './useConversations'

export const conversationKey = (id: string) => ['conversation', id] as const
export const membersKey = (id: string) => ['members', id] as const

function fromList(queryClient: QueryClient, id: string) {
    return queryClient
        .getQueryData<InfiniteData<ConversationSummary[]>>(conversationsKey)
        ?.pages.flat()
        .find((c) => c.id === id)
}

// карточка чата; пока загружается, берётся строка из списка - окно открывается без ожидания
export function useConversation(id: string) {
    const queryClient = useQueryClient()
    return useQuery({
        queryKey: conversationKey(id),
        queryFn: () => getConversation(id),
        placeholderData: () => fromList(queryClient, id),
    })
}

// участники с курсорами прочтения: от них зависят галочки у своих сообщений
export function useMembers(id: string) {
    return useQuery({ queryKey: membersKey(id), queryFn: () => getMembers(id) })
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
