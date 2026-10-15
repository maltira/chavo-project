import type { Member } from '@/entities/conversation'
import type { Message } from '@/entities/message'

import { firstUnreadId, readBoundary } from './readState'

const msg = (id: string) => ({ id }) as Message
const member = (user_id: string, last_read_message_id: string | null) =>
    ({ user_id, last_read_message_id }) as Member

describe('readBoundary', () => {
    // свежие первыми: m3 - последнее
    const messages = [msg('m3'), msg('m2'), msg('m1')]

    it('takes the freshest cursor of the others, ignoring mine', () => {
        expect(
            readBoundary(
                messages,
                [member('me', 'm3'), member('a', 'm1'), member('b', 'm2')],
                'me',
            ),
        ).toBe(1)
    })

    it('is undefined when nobody else has read the loaded messages', () => {
        expect(
            readBoundary(messages, [member('me', 'm3'), member('a', 'old')], 'me'),
        ).toBeUndefined()
        expect(readBoundary(messages, undefined, 'me')).toBeUndefined()
    })
})

describe('firstUnreadId', () => {
    const history = [
        { id: 'm5', sender_id: 'a' },
        { id: 'm4', sender_id: 'me' },
        { id: 'm3', sender_id: 'a', is_deleted: true },
        { id: 'm2', sender_id: 'a' },
        { id: 'm1', sender_id: 'a' },
    ] as Message[]

    it('counts only messages of others that are not deleted', () => {
        expect(firstUnreadId(history, 2, 'me')).toBe('m2')
        expect(firstUnreadId(history, 1, 'me')).toBe('m5')
    })

    it('stops at the oldest loaded one and is null without unread', () => {
        expect(firstUnreadId(history, 10, 'me')).toBe('m1')
        expect(firstUnreadId(history, 0, 'me')).toBeNull()
    })
})
