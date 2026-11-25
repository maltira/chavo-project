import { SAVED_NAME } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import type { Profile } from '@/entities/user'

// запрос из поля: без пробелов по краям и без ведущего @
export function normalizeQuery(raw: string): string {
    return raw.trim().replace(/^@+/, '')
}

// подходит ли мой чат под запрос: группа - по названию, «Избранное» - по этому слову,
// личный - по имени и username собеседника
export function matchChat(
    chat: ConversationSummary,
    query: string,
    peer: Profile | undefined,
): boolean {
    const q = query.toLowerCase()
    if (!q) {
        return false
    }
    if (chat.conversation_type === 'group') {
        return (chat.name ?? '').toLowerCase().includes(q)
    }
    if (chat.is_saved) {
        return SAVED_NAME.toLowerCase().includes(q)
    }
    return (
        !!peer &&
        (peer.display_name.toLowerCase().includes(q) || peer.username.toLowerCase().includes(q))
    )
}
