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
