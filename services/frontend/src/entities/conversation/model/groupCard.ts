import { create } from 'zustand'

import type { AvatarShape, Status } from '@/entities/user/@x/conversation'

import type { PublicGroup } from '../api/conversations'
import type { InviteInfo } from '../api/groups'

// группа, в которой я (ещё) не состою: из поиска или по ссылке на чат
export interface GroupPreview {
    id: string
    name: string
    description: string | null
    avatar_url: string | null
    members_count: number
    is_member: boolean
    visibility: 'public' | 'private'
    slug: string | null
    // pending - заявка в закрытую группу уже отправлена
    request_status: string | null
    status: Status | null
    avatar_shape: AvatarShape | null
    // код ссылки, по которой открыли закрытую группу: нужен для заявки
    code?: string
}

export type GroupCardTarget =
    // своя группа: сведения, настройки и участники по карточке чата
    { kind: 'chat'; id: string } | { kind: 'preview'; group: GroupPreview }

interface GroupCardState {
    // открытая в модалке группа; null - закрыта
    target: GroupCardTarget | null
    openChat: (id: string) => void
    openPreview: (group: GroupPreview) => void
    close: () => void
}

// модалка группы: открывается из поиска, меню и шапки чата и по ссылке, показывается на странице
export const useGroupCard = create<GroupCardState>()((set) => ({
    target: null,
    openChat: (id) => set({ target: { kind: 'chat', id } }),
    openPreview: (group) =>
        set({
            target: group.is_member ? { kind: 'chat', id: group.id } : { kind: 'preview', group },
        }),
    close: () => set({ target: null }),
}))

export function previewFromSearch(group: PublicGroup): GroupPreview {
    return { ...group, visibility: 'public', request_status: null }
}

export function previewFromInvite(info: InviteInfo, code: string): GroupPreview {
    return {
        id: info.conversation_id,
        name: info.name ?? 'Группа',
        description: info.description,
        avatar_url: info.avatar_url,
        members_count: info.members_count,
        is_member: info.is_member,
        visibility: info.visibility,
        slug: info.slug,
        request_status: info.request_status,
        status: info.status,
        avatar_shape: info.avatar_shape,
        code,
    }
}
