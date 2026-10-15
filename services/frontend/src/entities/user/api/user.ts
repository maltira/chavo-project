import { request } from '@/shared/api'

import type { Profile } from '../model/types'

// чужой профиль
export function getUser(userId: string): Promise<Profile> {
    return request<Profile>(`/users/${encodeURIComponent(userId)}`)
}

// поиск людей: сервер ищет по началу username
export function searchUsers(query: string): Promise<Profile[]> {
    return request<Profile[]>(`/users?q=${encodeURIComponent(query)}&limit=20`)
}

export interface BlockStatus {
    blocked_by_me: boolean
    blocked_by_them: boolean
}

export function getBlockStatus(userId: string): Promise<BlockStatus> {
    return request<BlockStatus>(`/users/${encodeURIComponent(userId)}/block-status`)
}

export function blockUser(userId: string): Promise<void> {
    return request<void>(`/users/${encodeURIComponent(userId)}/block`, { method: 'POST' })
}

export function unblockUser(userId: string): Promise<void> {
    return request<void>(`/users/${encodeURIComponent(userId)}/block`, { method: 'DELETE' })
}
