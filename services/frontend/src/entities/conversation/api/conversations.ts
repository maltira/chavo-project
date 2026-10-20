import { request } from '@/shared/api'

import type { AvatarShape, Status } from '@/entities/user/@x/conversation'

import type { ConversationSummary } from '../model/types'

// размер страницы списка; сервер отдаёт не больше 30 за запрос
export const CONVERSATIONS_PAGE = 30

export interface Page<T> {
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

// открытая группа в поиске (закрытые сервер не отдаёт)
export interface PublicGroup {
    id: string
    name: string
    slug: string
    description: string | null
    avatar_url: string | null
    members_count: number
    is_member: boolean
    status: Status | null
    avatar_shape: AvatarShape | null
}

export async function searchGroups(query: string): Promise<PublicGroup[]> {
    const page = await request<Page<PublicGroup>>(
        `/conversations/search?q=${encodeURIComponent(query)}&limit=20`,
    )
    return page.items
}

export function joinGroup(id: string): Promise<void> {
    return request<void>(`/conversations/${encodeURIComponent(id)}/join`, { method: 'POST' })
}

// участник выходит из группы, админ удаляет её для всех; личный чат так удалить нельзя
export function deleteConversation(id: string): Promise<void> {
    return request<void>(`/conversations/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export interface CreateGroupInput {
    name: string
    description?: string
    visibility: 'public' | 'private'
    // код ссылки на чат, обязателен для открытой группы
    slug?: string
    member_ids: string[]
}

// новая группа; создатель - её админ, member_ids добавляются сразу
export function createGroup(input: CreateGroupInput): Promise<ConversationSummary> {
    return request<ConversationSummary>('/conversations', { method: 'POST', body: input })
}
