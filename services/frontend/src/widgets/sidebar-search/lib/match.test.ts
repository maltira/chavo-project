import type { ConversationSummary } from '@/entities/conversation'
import type { Profile } from '@/entities/user'

import { matchChat, normalizeQuery } from './match'

const group = { conversation_type: 'group', name: 'Книжный клуб' } as ConversationSummary
const direct = { conversation_type: 'direct', name: null } as ConversationSummary
const peer = { display_name: 'Арефий Некрасов', username: 'arefiy' } as Profile

describe('sidebar search', () => {
    it('drops spaces and the leading @', () => {
        expect(normalizeQuery('  @@arren ')).toBe('arren')
    })

    it('matches groups by name and directs by the peer name or username', () => {
        expect(matchChat(group, 'клуб', undefined)).toBe(true)
        expect(matchChat(direct, 'некрас', peer)).toBe(true)
        expect(matchChat(direct, 'AREF', peer)).toBe(true)
        expect(matchChat(direct, 'зzz', peer)).toBe(false)
        // профиль собеседника ещё не загружен - не находим
        expect(matchChat(direct, 'aref', undefined)).toBe(false)
        expect(matchChat(group, '', undefined)).toBe(false)
    })
})
