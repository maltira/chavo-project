import type { ServerEvent } from '@/shared/api'

import type { Message } from '@/entities/message/@x/conversation'

import type { ChatSettings, ConversationSummary } from './types'

interface MessagePayload {
    conversation_id?: string
    message_id?: string
    sender_id?: string
    reader_id?: string
    reply_to_message_id?: string | null
    content?: string | null
    created_at?: string
    updated_at?: string
    kind?: Message['kind']
    system_event?: Message['system_event']
    target_id?: string | null
}

// результат применения события к списку:
//  list       - новый список (событие учтено на месте)
//  invalidate - данных события не хватает, список надо перечитать
//  ignore     - событие списка не касается
export type ListUpdate =
    { kind: 'list'; list: ConversationSummary[] } | { kind: 'invalidate' } | { kind: 'ignore' }

// порядок списка, как у сервера: закреплённые сверху (новый закреп выше), дальше свежие;
// время сравнивается числом - у строк сервера разная длина дробной части секунд
export const byListOrder = (a: ConversationSummary, b: ConversationSummary) =>
    (b.pinned_at ? Date.parse(b.pinned_at) : 0) - (a.pinned_at ? Date.parse(a.pinned_at) : 0) ||
    Date.parse(b.last_message_at) - Date.parse(a.last_message_at)

// события, после которых список перечитывается целиком: меняется состав чатов или их карточки
const RELOAD_EVENTS = new Set([
    'conversation.created',
    'conversation.updated',
    'conversation.deleted',
    'conversation.member.added',
    'conversation.member.removed',
    'conversation.join_request.approved',
    // удалённое последнее сообщение надо заменить предыдущим, а его в событии нет
    'message.deleted',
])

export function applyToConversations(
    list: ConversationSummary[] | undefined,
    event: ServerEvent,
    myId: string | undefined,
): ListUpdate {
    if (RELOAD_EVENTS.has(event.type)) {
        return { kind: 'invalidate' }
    }
    const data = (event.data ?? {}) as MessagePayload

    switch (event.type) {
        case 'message.created': {
            const index = list?.findIndex((c) => c.id === data.conversation_id) ?? -1
            if (!list || index < 0 || !data.message_id || !data.sender_id || !data.created_at) {
                // чат ещё не загружен (например, новый личный): перечитываем
                return { kind: 'invalidate' }
            }
            const chat = list[index]!
            const message: Message = {
                id: data.message_id,
                conversation_id: chat.id,
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
            }
            // системные сообщения поднимают чат, но в непрочитанные не идут (так считает и сервер)
            const counts = data.sender_id !== myId && message.kind !== 'system'
            const updated: ConversationSummary = {
                ...chat,
                last_message: message,
                last_message_at: data.created_at,
                last_message_read: false,
                unread_count: chat.unread_count + (counts ? 1 : 0),
            }
            const next = list.map((c, i) => (i === index ? updated : c))
            return { kind: 'list', list: next.sort(byListOrder) }
        }

        case 'message.updated': {
            if (!list) {
                return { kind: 'ignore' }
            }
            let changed = false
            const next = list.map((c) => {
                if (c.last_message?.id !== data.message_id) {
                    return c
                }
                changed = true
                return {
                    ...c,
                    last_message: {
                        ...c.last_message!,
                        content: data.content ?? c.last_message!.content,
                        is_edited: true,
                        updated_at: data.updated_at ?? c.last_message!.updated_at,
                    },
                }
            })
            return changed ? { kind: 'list', list: next } : { kind: 'ignore' }
        }

        case 'message.read': {
            // свой курсор меняет число непрочитанных: его считает сервер
            if (data.reader_id === myId) {
                return { kind: 'invalidate' }
            }
            const chat = list?.find((c) => c.id === data.conversation_id)
            if (!list || !chat) {
                return { kind: 'ignore' }
            }
            // курсор собеседника дошёл до последнего сообщения - галочки становятся двойными
            if (chat.last_message?.id === data.message_id) {
                return {
                    kind: 'list',
                    list: list.map((c) => (c === chat ? { ...c, last_message_read: true } : c)),
                }
            }
            return { kind: 'invalidate' }
        }

        default:
            return { kind: 'ignore' }
    }
}

// chat - чат с новыми настройками; settings - что пришло в conversation.settings.updated
interface SettingsPayload extends Partial<ChatSettings> {
    conversation_id?: string
}

export interface ListsUpdate {
    main?: ConversationSummary[]
    archived?: ConversationSummary[]
    // чата нет ни в одном загруженном списке - их надо перечитать
    reload: boolean
}

// новые личные настройки чата: чат обновляется на месте и при архивации переезжает между основным
// списком и архивом; незагруженный список (undefined) не трогается
export function applyChatSettings(
    main: ConversationSummary[] | undefined,
    archived: ConversationSummary[] | undefined,
    event: ServerEvent,
): ListsUpdate {
    const data = (event.data ?? {}) as SettingsPayload
    const chat =
        main?.find((c) => c.id === data.conversation_id) ??
        archived?.find((c) => c.id === data.conversation_id)
    if (!chat) {
        return { reload: true }
    }
    const updated: ConversationSummary = {
        ...chat,
        muted_until: data.muted_until ?? null,
        archived_at: data.archived_at ?? null,
        pinned_at: data.pinned_at ?? null,
    }
    const place = (list: ConversationSummary[] | undefined, inside: boolean) =>
        list &&
        [...list.filter((c) => c.id !== chat.id), ...(inside ? [updated] : [])].sort(byListOrder)
    return {
        main: place(main, !updated.archived_at),
        archived: place(archived, !!updated.archived_at),
        // чат ушёл в незагруженный список - тот загрузится с сервера, когда понадобится
        reload: false,
    }
}
