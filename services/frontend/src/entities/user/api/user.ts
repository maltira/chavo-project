import { photoForm, request } from '@/shared/api'

import type { AvatarShape, Profile, Status } from '../model/types'

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

export interface ProfileUpdate {
    display_name?: string
    username?: string
    bio?: string
}

export function updateMe(update: ProfileUpdate): Promise<void> {
    return request<void>('/users/me', { method: 'PATCH', body: update })
}

export interface StatusInput {
    icon: string
    text: string
    background: string
    foreground: string
    duration: string
}

// каждый вызов ставит новый статус целиком, срок считается от момента запроса
export function setMyStatus(input: StatusInput): Promise<Status> {
    return request<Status>('/users/me/status', { method: 'POST', body: input })
}

export function clearMyStatus(): Promise<void> {
    return request<void>('/users/me/status', { method: 'DELETE' })
}

export function setMyAvatarShape(shape: AvatarShape): Promise<AvatarShape> {
    return request<AvatarShape>('/users/me/avatar-shape', { method: 'POST', body: shape })
}

export function clearMyAvatarShape(): Promise<void> {
    return request<void>('/users/me/avatar-shape', { method: 'DELETE' })
}

// фото профиля: сервер проверяет и перекодирует файл и возвращает путь нового фото
export function uploadMyAvatar(photo: Blob): Promise<{ avatar_url: string }> {
    return request<{ avatar_url: string }>('/users/me/avatar', {
        method: 'POST',
        body: photoForm(photo),
    })
}

// убирает текущее фото; текущим становится предыдущее из истории
export function deleteMyAvatar(): Promise<void> {
    return request<void>('/users/me/avatar', { method: 'DELETE' })
}

// фото из истории аватарок: до 5 последних, первое (самое новое) - текущее
export interface AvatarPhoto {
    id: string
    avatar_url: string
    created_at: string
    is_current: boolean
}

// историю видят все, кто видит аватар; у заблокировавшего меня она пуста
export async function getAvatarPhotos(userId: string): Promise<AvatarPhoto[]> {
    const res = await request<{ items: AvatarPhoto[] }>(
        `/users/${encodeURIComponent(userId)}/avatars`,
    )
    return res.items
}

// сделать фото из истории главным (текущим)
export function selectAvatarPhoto(id: string): Promise<void> {
    return request<void>(`/users/me/avatars/${encodeURIComponent(id)}/select`, { method: 'POST' })
}

// убрать именно это фото из истории
export function deleteAvatarPhoto(id: string): Promise<void> {
    return request<void>(`/users/me/avatars/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export interface Settings {
    allow_group_invites: boolean
    show_online_status: boolean
    // в пересланных моих сообщениях вместо имени - «скрытый автор»
    hide_forward_author: boolean
    // избранные эмодзи панели в поле ввода: до 30, в порядке добавления
    favorite_emojis: string[]
}

// сколько эмодзи можно держать в избранном (так же ограничивает сервер)
export const MAX_FAVORITE_EMOJIS = 30

export function getSettings(): Promise<Settings> {
    return request<Settings>('/users/me/settings')
}

export function updateSettings(update: Partial<Settings>): Promise<Settings> {
    return request<Settings>('/users/me/settings', { method: 'PATCH', body: update })
}

export interface BlockedUser {
    blocked_user_id: string
    username: string
    display_name: string
    avatar_url?: string
    blocked_at: string
}

// размер страницы чёрного списка; сервер отдаёт не больше 100 за запрос
export const BLOCKED_PAGE = 50

export async function getBlocked(offset = 0): Promise<BlockedUser[]> {
    const page = await request<{ items: BlockedUser[] }>(
        `/users/me/blocked?limit=${BLOCKED_PAGE}&offset=${offset}`,
    )
    return page.items
}
