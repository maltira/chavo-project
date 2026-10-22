import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import type { ServerEvent } from '@/shared/api'

import type { Message, MessagesPage } from './types'

export const messagesKey = (conversationId: string) => ['messages', conversationId] as const

type Pages = InfiniteData<MessagesPage, string | undefined>

// правка загруженной истории; если история чата ещё не открывалась, кеша нет и менять нечего
function updatePages(
    queryClient: QueryClient,
    conversationId: string,
    change: (pages: MessagesPage[]) => MessagesPage[],
) {
    queryClient.setQueryData<Pages>(messagesKey(conversationId), (data) =>
        data ? { ...data, pages: change(data.pages) } : data,
    )
}

const mapMessages = (pages: MessagesPage[], fn: (m: Message) => Message) =>
    pages.map((page) => ({ ...page, items: page.items.map(fn) }))

// новое сообщение - в начало первой страницы (свежие сверху); повтор по id не добавляется
export function addMessage(queryClient: QueryClient, message: Message) {
    updatePages(queryClient, message.conversation_id, (pages) => {
        // уже есть (своё сообщение после ответа сервера): данные обновляются, ключ строки остаётся прежним
        if (pages.some((page) => page.items.some((m) => m.id === message.id))) {
            return mapMessages(pages, (m) =>
                m.id === message.id ? { ...message, local_key: m.local_key } : m,
            )
        }
        const [first, ...rest] = pages
        return first ? [{ ...first, items: [message, ...first.items] }, ...rest] : pages
    })
}

// замена временного сообщения ответом сервера; у настоящего остаётся ключ временного (local_key),
// чтобы строка в истории не пересоздавалась. Если событие успело раньше, временное убирается,
// а его ключ переходит к пришедшему по событию
export function replaceMessage(queryClient: QueryClient, tempId: string, message: Message) {
    updatePages(queryClient, message.conversation_id, (pages) => {
        const exists = pages.some((page) => page.items.some((m) => m.id === message.id))
        return pages.map((page) => ({
            ...page,
            items: exists
                ? page.items
                      .filter((m) => m.id !== tempId)
                      .map((m) => (m.id === message.id ? { ...m, local_key: tempId } : m))
                : page.items.map((m) => (m.id === tempId ? { ...message, local_key: tempId } : m)),
        }))
    })
}

export function removeMessage(queryClient: QueryClient, conversationId: string, id: string) {
    updatePages(queryClient, conversationId, (pages) =>
        pages.map((page) => ({ ...page, items: page.items.filter((m) => m.id !== id) })),
    )
}

export function patchMessage(
    queryClient: QueryClient,
    conversationId: string,
    id: string,
    patch: Partial<Message>,
) {
    updatePages(queryClient, conversationId, (pages) =>
        mapMessages(pages, (m) => (m.id === id ? { ...m, ...patch } : m)),
    )
}

interface MessageEventData {
    conversation_id?: string
    message_id?: string
    sender_id?: string
    reply_to_message_id?: string | null
    content?: string | null
    created_at?: string
    updated_at?: string
    kind?: Message['kind']
    system_event?: Message['system_event']
    target_id?: string | null
    is_forwarded?: boolean
    forwarded_from_user_id?: string | null
}

// события сообщений дописывают историю открытых чатов
export function applyMessageEvent(queryClient: QueryClient, event: ServerEvent) {
    const data = (event.data ?? {}) as MessageEventData
    const conversationId = data.conversation_id
    const id = data.message_id
    if (!conversationId || !id) {
        return
    }

    switch (event.type) {
        case 'message.created':
            if (data.sender_id && data.created_at) {
                addMessage(queryClient, {
                    id,
                    conversation_id: conversationId,
                    sender_id: data.sender_id,
                    content: data.content ?? null,
                    reply_to_message_id: data.reply_to_message_id ?? null,
                    is_edited: false,
                    is_deleted: false,
                    created_at: data.created_at,
                    updated_at: null,
                    kind: data.kind ?? 'user',
                    system_event: data.system_event ?? null,
                    target_id: data.target_id ?? null,
                    is_forwarded: data.is_forwarded ?? false,
                    forwarded_from_user_id: data.forwarded_from_user_id ?? null,
                })
            }
            break
        case 'message.updated':
            patchMessage(queryClient, conversationId, id, {
                content: data.content ?? null,
                is_edited: true,
                updated_at: data.updated_at ?? null,
            })
            break
        case 'message.deleted':
            patchMessage(queryClient, conversationId, id, { content: null, is_deleted: true })
            break
    }
}
