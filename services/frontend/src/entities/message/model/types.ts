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
    // только на клиенте: отправлено, но сервер ещё не ответил
    pending?: boolean
}

export interface MessagesPage {
    items: Message[]
    limit: number
    next_before: string | null
}
