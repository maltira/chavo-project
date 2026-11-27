import type { MessageEntity } from '@/shared/lib'

export interface MessageReaction {
    emoji: string
    count: number
    me: boolean
    user_ids: string[]
}

export interface LinkPreview {
    url: string
    title?: string | null
    description?: string | null
    image_url?: string | null
    site_name?: string | null
}

export interface Message {
    id: string
    conversation_id: string
    sender_id: string
    content: string | null // null у удалённого сообщения
    reply_to_message_id: string | null
    is_edited: boolean
    is_deleted: boolean
    created_at: string
    updated_at: string | null
    // system - событие чата: текста нет, его собирает клиент по system_event, sender_id (кто совершил)
    // и target_id (кого касается); у pinned закреплённое сообщение - reply_to_message_id; без поля - обычное
    kind?: 'user' | 'system'
    system_event?: SystemEvent | null
    target_id?: string | null
    // пересланная копия: forwarded_from_user_id - первый автор, null - автор скрыл имя;
    // пересланное «без указания автора» - обычное сообщение
    is_forwarded?: boolean
    forwarded_from_user_id?: string | null
    // форматирование текста (жирный, курсив, подчёркивание); без него - обычный текст
    entities?: MessageEntity[]
    // отправлено без звука (не показывает уведомление и не воспроизводит звук)
    is_silent?: boolean
    // агрегированные реакции сообщения
    reactions?: MessageReaction[]
    // превью ссылки (OpenGraph), если есть
    link_preview?: LinkPreview | null
    // только на клиенте: отправлено, но сервер ещё не ответил
    pending?: boolean
    // только на клиенте: id временного сообщения, которое заменил ответ сервера - ключ строки
    // в истории не меняется, и сообщение не появляется второй раз
    local_key?: string
}

// joined - вступил сам или по заявке, added - админ добавил, left - вышел сам, removed - админ исключил,
// pinned - закрепил сообщение для всех
export type SystemEvent = 'joined' | 'added' | 'left' | 'removed' | 'pinned'

export interface MessagesPage {
    items: Message[]
    limit: number
    next_before: string | null
    next_after?: string | null
    has_older?: boolean
    has_newer?: boolean
}
