import { request } from '@/shared/api'

import type { ConversationSummary } from '../model/types'

// размер страницы списка; сервер отдаёт не больше 30 за запрос
export const CONVERSATIONS_PAGE = 30

interface Page<T> {
    items: T[]
    limit: number
    offset: number
}

export async function getConversations(offset = 0): Promise<ConversationSummary[]> {
    const page = await request<Page<ConversationSummary>>(
        `/conversations?limit=${CONVERSATIONS_PAGE}&offset=${offset}`,
    )
    return page.items
}
