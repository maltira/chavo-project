import type { AvatarShape, Status } from '@/entities/user/@x/conversation'

// сообщение в том виде, в каком его отдаёт Gateway
export interface Message {
    id: string
    conversation_id: string
    sender_id: string
    // null у удалённого сообщения
    content: string | null
    reply_to_message_id: string | null
    is_edited: boolean
    is_deleted: boolean
    created_at: string
    updated_at: string | null
}

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
}

// строка списка чатов (GET /api/conversations)
export interface ConversationSummary extends Conversation {
    my_role: 'admin' | 'member'
    peer_id: string | null
    last_message: Message | null
    unread_count: number
    // последнее сообщение прочитал кто-то кроме его автора
    last_message_read: boolean
}
