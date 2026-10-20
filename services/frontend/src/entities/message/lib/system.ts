import { useUser } from '@/entities/user/@x/message'

import type { Message, SystemEvent } from '../model/types'

export function isSystem(message: Message): boolean {
    return message.kind === 'system'
}

// часть текста системного сообщения: имя участника (в истории - ссылка на профиль) или обычный текст
export type SystemPart =
    { type: 'user'; userId: string; name: string } | { type: 'text'; text: string }

// шаблон: actor - кто совершил, target - кого касается
const TEMPLATES: Record<SystemEvent, ('actor' | 'target' | string)[]> = {
    joined: ['actor', ' присоединился к чату'],
    added: ['actor', ' добавил ', 'target'],
    left: ['actor', ' покинул чат'],
    removed: ['actor', ' исключил ', 'target'],
}

export function systemParts(
    event: SystemEvent,
    actor: { userId: string; name: string },
    target: { userId: string; name: string },
): SystemPart[] {
    return TEMPLATES[event].map((piece) =>
        piece === 'actor'
            ? { type: 'user', ...actor }
            : piece === 'target'
              ? { type: 'user', ...target }
              : { type: 'text', text: piece },
    )
}

// части системного сообщения с актуальными именами (пока профиль не загружен - многоточие):
// при смене имени история тоже меняется
export function useSystemParts(message: Message | null | undefined): SystemPart[] {
    const system = !!message && isSystem(message) && !!message.system_event
    const actor = useUser(system ? message.sender_id : null)
    const targetId = system ? (message.target_id ?? null) : null
    const target = useUser(targetId)
    if (!system) {
        return []
    }
    return systemParts(
        message.system_event!,
        { userId: message.sender_id, name: actor.data?.display_name ?? '…' },
        { userId: targetId ?? '', name: target.data?.display_name ?? '…' },
    )
}

// то же одной строкой - для превью в списке чатов
export function useSystemText(message: Message | null | undefined): string {
    return useSystemParts(message)
        .map((part) => (part.type === 'user' ? part.name : part.text))
        .join('')
}
