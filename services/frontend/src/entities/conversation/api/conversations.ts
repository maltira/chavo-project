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

export function getConversation(id: string): Promise<ConversationSummary> {
    return request<ConversationSummary>(`/conversations/${encodeURIComponent(id)}`)
}

export interface Member {
    conversation_id: string
    user_id: string
    role: 'admin' | 'member'
    last_read_message_id: string | null
    joined_at: string
}

// предел страницы участников на сервере
export const MEMBERS_LIMIT = 100

export async function getMembers(id: string): Promise<Member[]> {
    const page = await request<Page<Member>>(
        `/conversations/${encodeURIComponent(id)}/members?limit=${MEMBERS_LIMIT}`,
    )
    return page.items
}
