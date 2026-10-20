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
    // system - событие состава группы: текста нет, его собирает клиент по system_event,
    // sender_id (кто совершил) и target_id (кого касается); без поля - обычное сообщение
    kind?: 'user' | 'system'
    system_event?: SystemEvent | null
    target_id?: string | null
    // только на клиенте: отправлено, но сервер ещё не ответил
    pending?: boolean
    // только на клиенте: id временного сообщения, которое заменил ответ сервера - ключ строки
    // в истории не меняется, и сообщение не появляется второй раз
    local_key?: string
}

// joined - вступил сам или по заявке, added - админ добавил, left - вышел сам, removed - админ исключил
export type SystemEvent = 'joined' | 'added' | 'left' | 'removed'

export interface MessagesPage {
    items: Message[]
    limit: number
    next_before: string | null
}
