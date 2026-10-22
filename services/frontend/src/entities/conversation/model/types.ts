import type { Message } from '@/entities/message/@x/conversation'
import type { AvatarShape, Status } from '@/entities/user/@x/conversation'

export interface Conversation {
    id: string
    conversation_type: 'direct' | 'group'
    visibility: 'public' | 'private'
    // у личного чата имени и аватара нет: они берутся из профиля собеседника
    name: string | null
    description: string | null
    avatar_url: string | null
    last_message_at: string
    created_at: string
    updated_at: string
    status: Status | null
    avatar_shape: AvatarShape | null
    // код ссылки на чат открытой группы; у закрытых и личных - null
    slug: string | null
}

// строка списка чатов (GET /api/conversations)
export interface ConversationSummary extends Conversation {
    my_role: 'admin' | 'member'
    peer_id: string | null
    last_message: Message | null
    unread_count: number
    // последнее сообщение прочитал кто-то кроме его автора
    last_message_read: boolean
    members_count: number
    // код ссылки закрытой группы: только у её админов и только в карточке чата
    invite_code?: string | null
    // личные настройки чата: видит и меняет только сам участник
    muted_until: string | null
    archived_at: string | null
    pinned_at: string | null
}

export type ChatSettings = Pick<ConversationSummary, 'muted_until' | 'archived_at' | 'pinned_at'>

// без звука, пока срок не прошёл: по окончании звук включается сам, без запроса к серверу
export function isMuted(chat: Pick<ChatSettings, 'muted_until'>, now = Date.now()) {
    return !!chat.muted_until && Date.parse(chat.muted_until) > now
}
