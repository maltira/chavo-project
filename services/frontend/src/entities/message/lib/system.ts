import { useUser } from '@/entities/user/@x/message'

import type { Message, SystemEvent } from '../model/types'
import { useMessage } from '../model/useMessage'

export function isSystem(message: Message): boolean {
    return message.kind === 'system'
}

// часть текста системного сообщения: имя участника (в истории - ссылка на профиль), начало закреплённого
// сообщения (в истории - переход к нему) или обычный текст
export type SystemPart =
    | { type: 'user'; userId: string; name: string }
    | { type: 'quote'; messageId: string; text: string }
    | { type: 'text'; text: string }

// шаблон: actor - кто совершил, target - кого касается, quote - закреплённое сообщение
const TEMPLATES: Record<SystemEvent, ('actor' | 'target' | 'quote' | string)[]> = {
    joined: ['actor', ' присоединился к чату'],
    added: ['actor', ' добавил ', 'target'],
    left: ['actor', ' покинул чат'],
    removed: ['actor', ' исключил ', 'target'],
    pinned: ['actor', ' закрепил сообщение ', 'quote'],
}

// сколько символов закреплённого сообщения показывать в строке «закрепил»
const QUOTE_LENGTH = 40

// начало текста по границе слова: «Завтра встречаемся у входа…»
export function quotePreview(message: Message | null | undefined): string {
    if (!message) {
        return '…'
    }
    if (message.is_deleted || message.content === null) {
        return 'удалённое сообщение'
    }
    const text = message.content.replace(/\s+/g, ' ').trim()
    if (text.length <= QUOTE_LENGTH) {
        return `«${text}»`
    }
    const cut = text.slice(0, QUOTE_LENGTH)
    const space = cut.lastIndexOf(' ')
    return `«${(space > QUOTE_LENGTH / 2 ? cut.slice(0, space) : cut).trimEnd()}…»`
}

export function systemParts(
    event: SystemEvent,
    actor: { userId: string; name: string },
    target: { userId: string; name: string },
    quote: { messageId: string; text: string } = { messageId: '', text: '…' },
): SystemPart[] {
    return TEMPLATES[event].map((piece) =>
        piece === 'actor'
            ? { type: 'user', ...actor }
            : piece === 'target'
              ? { type: 'user', ...target }
              : piece === 'quote'
                ? { type: 'quote', ...quote }
                : { type: 'text', text: piece },
    )
}

// части системного сообщения с актуальными именами (пока профиль не загружен - многоточие):
// при смене имени история тоже меняется; quoted - закреплённое сообщение, если оно уже есть в истории
// (иначе загружается отдельно)
export function useSystemParts(
    message: Message | null | undefined,
    quoted?: Message,
): SystemPart[] {
    const system = !!message && isSystem(message) && !!message.system_event
    const actor = useUser(system ? message.sender_id : null)
    const targetId = system ? (message.target_id ?? null) : null
    const target = useUser(targetId)
    const quoteId =
        system && message.system_event === 'pinned' ? (message.reply_to_message_id ?? null) : null
    const fetched = useMessage(quoteId && !quoted ? quoteId : null)
    if (!system) {
        return []
    }
    return systemParts(
        message.system_event!,
        { userId: message.sender_id, name: actor.data?.display_name ?? '…' },
        { userId: targetId ?? '', name: target.data?.display_name ?? '…' },
        // строку закреплённого сообщения могли удалить вместе с ним
        {
            messageId: quoteId ?? '',
            text: quoteId ? quotePreview(quoted ?? fetched.data) : 'удалённое сообщение',
        },
    )
}

// то же одной строкой - для превью в списке чатов
export function useSystemText(message: Message | null | undefined): string {
    return useSystemParts(message)
        .map((part) => (part.type === 'user' ? part.name : part.text))
        .join('')
}
