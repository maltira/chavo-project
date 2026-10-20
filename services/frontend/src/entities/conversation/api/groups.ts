import { request } from '@/shared/api'

import type { AvatarShape, Status, StatusInput } from '@/entities/user/@x/conversation'

import type { ConversationSummary } from '../model/types'
import type { Member, Page } from './conversations'

const base = (id: string) => `/conversations/${encodeURIComponent(id)}`

// страница списков в окне группы: участники, заявки, заблокированные
export const GROUP_PAGE = 50

export interface GroupUpdate {
    name?: string
    // пустая строка очищает описание
    description?: string
    visibility?: 'public' | 'private'
    slug?: string
}

// ответ без последнего сообщения и счётчиков: после правки карточку перечитываем
export function updateGroup(id: string, patch: GroupUpdate): Promise<ConversationSummary> {
    return request<ConversationSummary>(base(id), { method: 'PATCH', body: patch })
}

// новый код ссылки закрытой группы; старая ссылка перестаёт работать
export async function regenerateInvite(id: string): Promise<string> {
    const res = await request<{ invite_token: string }>(`${base(id)}/invite-link/regenerate`, {
        method: 'POST',
    })
    return res.invite_token
}

// группа по коду из ссылки на чат (/chat/<код>)
export interface InviteInfo {
    conversation_id: string
    visibility: 'public' | 'private'
    slug: string | null
    name: string | null
    description: string | null
    avatar_url: string | null
    members_count: number
    is_member: boolean
    // pending - заявка уже отправлена
    request_status: string | null
    status: Status | null
    avatar_shape: AvatarShape | null
}

export function getInviteInfo(code: string): Promise<InviteInfo> {
    return request<InviteInfo>(`/invites/${encodeURIComponent(code)}`)
}

export interface JoinRequest {
    id: string
    conversation_id: string
    user_id: string
    status: 'pending' | 'approved' | 'rejected'
    created_at: string
    updated_at: string
}

// заявка в закрытую группу: код ссылки подтверждает, что пользователя пригласили
export function requestJoin(id: string, code: string): Promise<JoinRequest> {
    return request<JoinRequest>(`${base(id)}/join-requests`, {
        method: 'POST',
        body: { invite_token: code },
    })
}

export async function getMembersPage(id: string, offset: number): Promise<Member[]> {
    const page = await request<Page<Member>>(
        `${base(id)}/members?limit=${GROUP_PAGE}&offset=${offset}`,
    )
    return page.items
}

export function addMembers(id: string, userIds: string[]): Promise<void> {
    return request<void>(`${base(id)}/members`, { method: 'POST', body: { user_ids: userIds } })
}

export function setMemberRole(id: string, userId: string, role: Member['role']): Promise<void> {
    return request<void>(`${base(id)}/members/${encodeURIComponent(userId)}`, {
        method: 'POST',
        body: { role },
    })
}

// исключение участника; ban - ещё и запрет возвращаться
export function removeMember(id: string, userId: string, ban: boolean): Promise<void> {
    return request<void>(
        `${base(id)}/members/${encodeURIComponent(userId)}${ban ? '?ban=true' : ''}`,
        { method: 'DELETE' },
    )
}

export async function getJoinRequests(id: string, offset: number): Promise<JoinRequest[]> {
    const page = await request<Page<JoinRequest>>(
        `${base(id)}/join-requests?status=pending&limit=${GROUP_PAGE}&offset=${offset}`,
    )
    return page.items
}

export function approveJoinRequest(id: string, requestId: string): Promise<void> {
    return request<void>(`${base(id)}/join-requests/${encodeURIComponent(requestId)}/approve`, {
        method: 'POST',
    })
}

export function rejectJoinRequest(id: string, requestId: string): Promise<void> {
    return request<void>(`${base(id)}/join-requests/${encodeURIComponent(requestId)}/reject`, {
        method: 'POST',
    })
}

export interface Ban {
    user_id: string
    banned_by: string
    created_at: string
}

export async function getBans(id: string, offset: number): Promise<Ban[]> {
    const page = await request<Page<Ban>>(`${base(id)}/bans?limit=${GROUP_PAGE}&offset=${offset}`)
    return page.items
}

export function unban(id: string, userId: string): Promise<void> {
    return request<void>(`${base(id)}/bans/${encodeURIComponent(userId)}`, { method: 'DELETE' })
}

// статус и фигура группы ставятся целиком; меняет только админ, событий об этом нет
export function setGroupStatus(id: string, input: StatusInput): Promise<Status> {
    return request<Status>(`${base(id)}/status`, { method: 'POST', body: input })
}

export function clearGroupStatus(id: string): Promise<void> {
    return request<void>(`${base(id)}/status`, { method: 'DELETE' })
}

export function setGroupAvatarShape(id: string, shape: AvatarShape): Promise<AvatarShape> {
    return request<AvatarShape>(`${base(id)}/avatar-shape`, { method: 'POST', body: shape })
}

export function clearGroupAvatarShape(id: string): Promise<void> {
    return request<void>(`${base(id)}/avatar-shape`, { method: 'DELETE' })
}
