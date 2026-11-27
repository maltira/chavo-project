import { request } from '@/shared/api'
import type { MessageEntity } from '@/shared/lib'

import type { LinkPreview, Message, MessageReaction, MessagesPage } from '../model/types'

// размер страницы истории; сервер отдаёт не больше 100
export const MESSAGES_PAGE = 50

// предел длины текста на сервере
export const MAX_MESSAGE_LENGTH = 4096

export type MessagesCursor =
    | { kind: 'before'; id: string }
    | { kind: 'after'; id: string }
    | { kind: 'around'; id: string }

export function getMessages(
    conversationId: string,
    cursor?: string | MessagesCursor,
): Promise<MessagesPage> {
    const params = new URLSearchParams({
        conversation_id: conversationId,
        limit: String(MESSAGES_PAGE),
    })
    if (typeof cursor === 'string') {
        params.set('before', cursor)
    } else if (cursor) {
        params.set(cursor.kind, cursor.id)
    }
    return request<MessagesPage>(`/messages?${params}`)
}

// в существующий чат - conversation_id; первое сообщение человеку - recipient_id (чат создаст сервер)
export interface SendMessageInput {
    conversation_id?: string
    recipient_id?: string
    content: string
    reply_to_message_id?: string | null
    // форматирование текста; диапазоны в единицах UTF-16
    entities?: MessageEntity[]
    // отправить без уведомления и звука
    is_silent?: boolean
    // не прикреплять превью ссылки к сообщению
    disable_link_preview?: boolean
}

export function sendMessage(input: SendMessageInput): Promise<Message> {
    return request<Message>('/messages', { method: 'POST', body: input })
}

// текст и форматирование заменяются целиком
export function editMessage(
    id: string,
    content: string,
    entities: MessageEntity[] = [],
    disableLinkPreview = false,
): Promise<Message> {
    return request<Message>(`/messages/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: {
            content,
            entities,
            ...(disableLinkPreview ? { disable_link_preview: true } : {}),
        },
    })
}

export async function getLinkPreview(url: string): Promise<LinkPreview | null> {
    const params = new URLSearchParams({ url })
    const res = await request<{ preview: LinkPreview | null }>(`/link-preview?${params}`)
    return res.preview
}

// не больше 20 сообщений одного чата и 5 чатов (вместе с «Избранным») за раз
export const MAX_FORWARD_MESSAGES = 20
export const MAX_FORWARD_CHATS = 5

export interface ForwardInput {
    message_ids: string[]
    conversation_ids: string[]
    // также в «Избранное»: сервер создаст его, если его ещё нет
    to_saved: boolean
    // без указания автора: копии - мои обычные сообщения
    hide_author: boolean
}

export interface ForwardedChat {
    conversation_id: string
    messages: Message[]
}

export async function forwardMessages(input: ForwardInput): Promise<ForwardedChat[]> {
    const res = await request<{ items: ForwardedChat[] }>('/messages/forward', {
        method: 'POST',
        body: input,
    })
    return res.items
}

// одно сообщение: видимое мне или то, на которое ссылается видимое (цитата, закреп)
export function getMessage(id: string): Promise<Message> {
    return request<Message>(`/messages/${encodeURIComponent(id)}`)
}

export function deleteMessage(id: string): Promise<void> {
    return request<void>(`/messages/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function deleteMessages(messageIds: string[]): Promise<void> {
    return request<void>('/messages/delete', {
        method: 'POST',
        body: { message_ids: messageIds },
    })
}

// курсор прочтения: всё до этого сообщения включительно прочитано
export function markRead(conversationId: string, messageId: string): Promise<void> {
    return request<void>(`/conversations/${encodeURIComponent(conversationId)}/read`, {
        method: 'POST',
        body: { message_id: messageId },
    })
}

// участники, чей курсор прочтения дошёл до сообщения (без его автора)
export async function getMessageReaders(
    conversationId: string,
    messageId: string,
): Promise<string[]> {
    const res = await request<{ user_ids: string[] }>(
        `/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/readers`,
    )
    return res.user_ids
}

export async function addReaction(messageId: string, emoji: string): Promise<MessageReaction[]> {
    const res = await request<{ reactions: MessageReaction[] }>(
        `/messages/${encodeURIComponent(messageId)}/reactions`,
        { method: 'POST', body: { emoji } },
    )
    return res.reactions
}

export async function removeReaction(messageId: string, emoji: string): Promise<MessageReaction[]> {
    const params = new URLSearchParams({ emoji })
    const res = await request<{ reactions: MessageReaction[] }>(
        `/messages/${encodeURIComponent(messageId)}/reactions?${params}`,
        { method: 'DELETE' },
    )
    return res.reactions
}
