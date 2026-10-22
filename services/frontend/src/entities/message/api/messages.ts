import { request } from '@/shared/api'

import type { Message, MessagesPage } from '../model/types'

// размер страницы истории; сервер отдаёт не больше 100
export const MESSAGES_PAGE = 50

// предел длины текста на сервере
export const MAX_MESSAGE_LENGTH = 4096

export function getMessages(conversationId: string, before?: string): Promise<MessagesPage> {
    const params = new URLSearchParams({
        conversation_id: conversationId,
        limit: String(MESSAGES_PAGE),
    })
    if (before) {
        params.set('before', before)
    }
    return request<MessagesPage>(`/messages?${params}`)
}

// в существующий чат - conversation_id; первое сообщение человеку - recipient_id (чат создаст сервер)
export interface SendMessageInput {
    conversation_id?: string
    recipient_id?: string
    content: string
    reply_to_message_id?: string | null
}

export function sendMessage(input: SendMessageInput): Promise<Message> {
    return request<Message>('/messages', { method: 'POST', body: input })
}

export function editMessage(id: string, content: string): Promise<Message> {
    return request<Message>(`/messages/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: { content },
    })
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

// курсор прочтения: всё до этого сообщения включительно прочитано
export function markRead(conversationId: string, messageId: string): Promise<void> {
    return request<void>(`/conversations/${encodeURIComponent(conversationId)}/read`, {
        method: 'POST',
        body: { message_id: messageId },
    })
}
