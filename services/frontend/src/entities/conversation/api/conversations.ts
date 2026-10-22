import { request } from '@/shared/api'

import type { AvatarShape, Status } from '@/entities/user/@x/conversation'

import type { ChatSettings, ConversationSummary } from '../model/types'

// размер страницы списка; сервер отдаёт не больше 30 за запрос
export const CONVERSATIONS_PAGE = 30

export interface Page<T> {
    items: T[]
    limit: number
    offset: number
}

// archived - архив; основной список архивных чатов не содержит
export async function getConversations(
    offset = 0,
    archived = false,
): Promise<ConversationSummary[]> {
    const page = await request<Page<ConversationSummary>>(
        `/conversations?limit=${CONVERSATIONS_PAGE}&offset=${offset}${archived ? '&archived=true' : ''}`,
    )
    return page.items
}

export type ChatSettingsResponse = ChatSettings & { conversation_id: string }

const chatPath = (id: string, action: string) =>
    `/conversations/${encodeURIComponent(id)}/${action}`

// duration - ключ из каталога (mute_durations в GET /customization)
export function muteChat(id: string, duration: string) {
    return request<ChatSettingsResponse>(chatPath(id, 'mute'), {
        method: 'POST',
        body: { duration },
    })
}

export function unmuteChat(id: string) {
    return request<ChatSettingsResponse>(chatPath(id, 'mute'), { method: 'DELETE' })
}

// архивация снимает закреп
export function setChatArchived(id: string, archived: boolean) {
    return request<ChatSettingsResponse>(chatPath(id, 'archive'), {
        method: archived ? 'POST' : 'DELETE',
    })
}

// закреп (до 5 чатов, иначе PIN_LIMIT) достаёт чат из архива
export function setChatPinned(id: string, pinned: boolean) {
    return request<ChatSettingsResponse>(chatPath(id, 'pin'), {
        method: pinned ? 'POST' : 'DELETE',
    })
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

// участник выходит из группы, админ удаляет её для всех; личный чат удаляется у себя (история скрыта,
// чат вернётся со следующим сообщением) или forEveryone - у обоих
export function deleteConversation(id: string, forEveryone = false): Promise<void> {
    return request<void>(
        `/conversations/${encodeURIComponent(id)}${forEveryone ? '?scope=all' : ''}`,
        { method: 'DELETE' },
    )
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
