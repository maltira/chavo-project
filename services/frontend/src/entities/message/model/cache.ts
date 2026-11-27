import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import type { ServerEvent } from '@/shared/api'
import type { MessageEntity } from '@/shared/lib'

import { MESSAGES_PAGE } from '../api/messages'
import type { MessagesCursor } from '../api/messages'
import type { Message, MessageReaction, MessagesPage } from './types'

export const messagesKey = (conversationId: string) => ['messages', conversationId] as const

export type MessagesPageParam = string | MessagesCursor | undefined
type Pages = InfiniteData<MessagesPage, MessagesPageParam>

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

// очистка кеша истории после ClearHistory (без удаления чата)
export function clearMessagesCache(queryClient: QueryClient, conversationId: string) {
    queryClient.setQueryData<Pages>(messagesKey(conversationId), (data) =>
        data
            ? {
                  pages: [
                      {
                          items: [],
                          limit: MESSAGES_PAGE,
                          next_before: null,
                          next_after: null,
                          has_older: false,
                          has_newer: false,
                      },
                  ],
                  pageParams: [undefined],
              }
            : data,
    )
}

// проверка, открыт ли чат в старом окне истории (не у нижнего края)
export function hasUnloadedNewerMessages(queryClient: QueryClient, conversationId: string): boolean {
    const data = queryClient.getQueryData<Pages>(messagesKey(conversationId))
    return !!data?.pages[0]?.has_newer
}

// новое сообщение - в начало первой страницы (свежие сверху); повтор по id не добавляется.
// если открыт фрагмент старой истории (has_newer), живые сообщения не смешиваем с ним - пользователь
// увидит их при возврате к последним сообщениям
export function addMessage(queryClient: QueryClient, message: Message) {
    updatePages(queryClient, message.conversation_id, (pages) => {
        // уже есть (своё сообщение после ответа сервера): данные обновляются, ключ строки остаётся прежним
        if (pages.some((page) => page.items.some((m) => m.id === message.id))) {
            return mapMessages(pages, (m) =>
                m.id === message.id ? { ...message, local_key: m.local_key } : m,
            )
        }
        const [first, ...rest] = pages
        if (!first || first.has_newer) {
            return pages
        }
        return [{ ...first, items: [message, ...first.items] }, ...rest]
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

interface ReactionEventSummary {
    emoji: string
    count: number
    user_ids: string[]
}

interface MessageEventData {
    conversation_id?: string
    message_id?: string
    sender_id?: string
    user_id?: string
    reply_to_message_id?: string | null
    content?: string | null
    created_at?: string
    updated_at?: string
    kind?: Message['kind']
    system_event?: Message['system_event']
    target_id?: string | null
    is_forwarded?: boolean
    forwarded_from_user_id?: string | null
    entities?: MessageEntity[]
    is_silent?: boolean
    emoji?: string
    added?: boolean
    reactions?: ReactionEventSummary[]
}

// события сообщений дописывают историю открытых чатов
export function applyMessageEvent(
    queryClient: QueryClient,
    event: ServerEvent,
    myId?: string,
) {
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
                    entities: data.entities ?? [],
                    is_silent: data.is_silent ?? false,
                    reactions: [],
                })
            }
            break
        case 'message.updated':
            patchMessage(queryClient, conversationId, id, {
                content: data.content ?? null,
                // новая разметка целиком: у правки без форматирования список пуст
                entities: data.entities ?? [],
                is_edited: true,
                updated_at: data.updated_at ?? null,
            })
            break
        case 'message.deleted':
            patchMessage(queryClient, conversationId, id, {
                content: null,
                is_deleted: true,
                reactions: [],
            })
            break
        case 'message.reaction.updated':
            updatePages(queryClient, conversationId, (pages) =>
                mapMessages(pages, (m) => {
                    if (m.id !== id) {
                        return m
                    }
                    const prevByEmoji = new Map((m.reactions ?? []).map((r) => [r.emoji, r]))
                    const nextReactions: MessageReaction[] = (data.reactions ?? []).map((r) => {
                        let me: boolean
                        if (myId && data.user_id === myId && r.emoji === data.emoji) {
                            me = !!data.added
                        } else if (prevByEmoji.has(r.emoji)) {
                            me = prevByEmoji.get(r.emoji)!.me
                        } else {
                            me = !!myId && r.user_ids.includes(myId)
                        }
                        return {
                            emoji: r.emoji,
                            count: r.count,
                            me,
                            user_ids: r.user_ids,
                        }
                    })
                    return { ...m, reactions: nextReactions }
                }),
            )
            break
    }
}
