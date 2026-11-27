import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import type { InfiniteData } from '@tanstack/react-query'
import { useCallback } from 'react'

import { getMessages } from '../api/messages'
import { messagesKey } from './cache'
import type { MessagesPageParam } from './cache'
import type { Message, MessagesPage } from './types'

function uniqueMessages(items: Message[]): Message[] {
    const seen = new Set<string>()
    return items.filter((m) => !seen.has(m.id) && seen.add(m.id))
}

export function useMessages(conversationId: string) {
    const queryClient = useQueryClient()
    const query = useInfiniteQuery({
        queryKey: messagesKey(conversationId),
        queryFn: ({ pageParam }) => getMessages(conversationId, pageParam),
        initialPageParam: undefined as MessagesPageParam,
        getNextPageParam: (last) =>
            last.next_before ? ({ kind: 'before', id: last.next_before } as const) : undefined,
        getPreviousPageParam: (first) =>
            first.has_newer && first.next_after
                ? ({ kind: 'after', id: first.next_after } as const)
                : undefined,
        // события дописывают историю сами; перечитывание при каждом открытии не нужно
        staleTime: Infinity,
    })

    // загрузка окна истории вокруг далёкого сообщения одним запросом (around)
    const loadAround = useCallback(
        async (messageId: string): Promise<boolean> => {
            try {
                const page = await getMessages(conversationId, { kind: 'around', id: messageId })
                if (!page.items.some((m) => m.id === messageId)) {
                    return false
                }
                queryClient.setQueryData<InfiniteData<MessagesPage, MessagesPageParam>>(
                    messagesKey(conversationId),
                    {
                        pages: [page],
                        pageParams: [{ kind: 'around', id: messageId }],
                    },
                )
                return true
            } catch {
                return false
            }
        },
        [conversationId, queryClient],
    )

    // возврат к последним сообщениям после перехода в глубь истории
    const resetToLatest = useCallback(async () => {
        const page = await getMessages(conversationId)
        queryClient.setQueryData<InfiniteData<MessagesPage, MessagesPageParam>>(
            messagesKey(conversationId),
            {
                pages: [page],
                pageParams: [undefined],
            },
        )
    }, [conversationId, queryClient])

    const items = query.data ? uniqueMessages(query.data.pages.flatMap((page) => page.items)) : undefined

    return {
        ...query,
        data: items,
        hasNewer: !!query.data?.pages[0]?.has_newer,
        loadAround,
        resetToLatest,
    }
}
