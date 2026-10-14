import { useInfiniteQuery } from '@tanstack/react-query'
import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import type { ServerEvent } from '@/shared/api'

import { CONVERSATIONS_PAGE, getConversations } from '../api/conversations'
import { applyToConversations } from './events'
import type { ConversationSummary } from './types'

export const conversationsKey = ['conversations'] as const

type Pages = InfiniteData<ConversationSummary[], number>

// список чатов, свежие сверху; страницы подгружаются по мере прокрутки,
// между загрузками его обновляют события WebSocket
export function useConversations() {
    const query = useInfiniteQuery({
        queryKey: conversationsKey,
        queryFn: ({ pageParam }) => getConversations(pageParam),
        initialPageParam: 0,
        // неполная страница - это конец списка; иначе следующая начинается после загруженных
        getNextPageParam: (last, all) =>
            last.length < CONVERSATIONS_PAGE ? undefined : all.flat().length,
    })

    return {
        ...query,
        // повтор возможен, когда событие подвинуло чат между страницами: оставляем первое вхождение
        data: query.data ? uniqueById(query.data.pages.flat()) : undefined,
    }
}

function uniqueById(list: ConversationSummary[]) {
    const seen = new Set<string>()
    return list.filter((c) => !seen.has(c.id) && seen.add(c.id))
}

function toPages(list: ConversationSummary[]): Pages {
    const pages: ConversationSummary[][] = []
    for (let i = 0; i < list.length; i += CONVERSATIONS_PAGE) {
        pages.push(list.slice(i, i + CONVERSATIONS_PAGE))
    }
    return { pages, pageParams: pages.map((_, i) => i * CONVERSATIONS_PAGE) }
}

// событие меняет список в кеше: новое сообщение поднимает чат наверх, прочтение ставит галочки
export function applyConversationEvent(
    queryClient: QueryClient,
    event: ServerEvent,
    myId: string | undefined,
) {
    const cached = queryClient.getQueryData<Pages>(conversationsKey)
    const list = cached ? uniqueById(cached.pages.flat()) : undefined
    const update = applyToConversations(list, event, myId)

    if (update.kind === 'list') {
        // число чатов не меняется, поэтому границы страниц остаются полными
        queryClient.setQueryData<Pages>(conversationsKey, toPages(update.list))
    } else if (update.kind === 'invalidate') {
        void queryClient.invalidateQueries({ queryKey: conversationsKey })
    }
}
