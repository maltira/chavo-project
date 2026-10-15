import type { Member } from '@/entities/conversation'
import type { Message } from '@/entities/message'

// индекс самого свежего сообщения, до которого дочитал кто-то кроме меня (история - свежие первыми);
// сообщение с индексом >= границы прочитано. undefined - никто ничего из загруженного не прочитал
export function readBoundary(
    messages: Message[],
    members: Member[] | undefined,
    myId: string | undefined,
): number | undefined {
    let boundary: number | undefined
    for (const member of members ?? []) {
        if (member.user_id === myId || !member.last_read_message_id) {
            continue
        }
        const index = messages.findIndex((m) => m.id === member.last_read_message_id)
        if (index >= 0 && (boundary === undefined || index < boundary)) {
            boundary = index
        }
    }
    return boundary
}

// самое старое из непрочитанных: unread - сколько чужих неудалённых сообщений не прочитано
// (так считает сервер); если загружено меньше, разделитель встаёт над самым старым из них
export function firstUnreadId(
    messages: Message[],
    unread: number,
    myId: string | undefined,
): string | null {
    if (unread <= 0) {
        return null
    }
    let found: string | null = null
    let counted = 0
    for (const message of messages) {
        if (message.sender_id === myId || message.is_deleted) {
            continue
        }
        found = message.id
        counted++
        if (counted === unread) {
            break
        }
    }
    return found
}
