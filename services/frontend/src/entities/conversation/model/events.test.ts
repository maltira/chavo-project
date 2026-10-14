import { applyToConversations } from './events'
import type { ConversationSummary } from './types'

function chat(
    id: string,
    at: string,
    extra: Partial<ConversationSummary> = {},
): ConversationSummary {
    return {
        id,
        conversation_type: 'direct',
        visibility: 'private',
        name: null,
        description: null,
        avatar_url: null,
        last_message_at: at,
        created_at: at,
        updated_at: at,
        status: null,
        avatar_shape: null,
        my_role: 'member',
        peer_id: 'peer',
        last_message: null,
        unread_count: 0,
        last_message_read: false,
        ...extra,
    }
}

const ME = 'me'

describe('applyToConversations', () => {
    const list = [chat('a', '2026-10-14T10:00:00Z'), chat('b', '2026-10-14T09:00:00Z')]

    it('puts a new message on top and counts it as unread', () => {
        const update = applyToConversations(
            list,
            {
                type: 'message.created',
                data: {
                    conversation_id: 'b',
                    message_id: 'm1',
                    sender_id: 'peer',
                    content: 'привет',
                    created_at: '2026-10-14T11:00:00Z',
                },
            },
            ME,
        )
        expect(update.kind).toBe('list')
        if (update.kind !== 'list') return
        expect(update.list.map((c) => c.id)).toEqual(['b', 'a'])
        expect(update.list[0]).toMatchObject({
            unread_count: 1,
            last_message: { id: 'm1', content: 'привет' },
        })
    })

    it('does not count my own message as unread and resets the read mark', () => {
        const read = [chat('a', '2026-10-14T10:00:00Z', { last_message_read: true })]
        const update = applyToConversations(
            read,
            {
                type: 'message.created',
                data: {
                    conversation_id: 'a',
                    message_id: 'm2',
                    sender_id: ME,
                    content: 'ещё',
                    created_at: '2026-10-14T11:00:00Z',
                },
            },
            ME,
        )
        if (update.kind !== 'list') throw new Error(update.kind)
        expect(update.list[0]).toMatchObject({ unread_count: 0, last_message_read: false })
    })

    it('reloads the list when the chat is not loaded yet', () => {
        const update = applyToConversations(
            list,
            {
                type: 'message.created',
                data: {
                    conversation_id: 'new',
                    message_id: 'm',
                    sender_id: 'x',
                    created_at: '2026-10-14T11:00:00Z',
                },
            },
            ME,
        )
        expect(update.kind).toBe('invalidate')
    })

    it('marks my last message as read when the peer cursor reaches it', () => {
        const mine = [
            chat('a', '2026-10-14T10:00:00Z', {
                last_message: {
                    id: 'm1',
                    conversation_id: 'a',
                    sender_id: ME,
                    content: 'x',
                    reply_to_message_id: null,
                    is_edited: false,
                    is_deleted: false,
                    created_at: '2026-10-14T10:00:00Z',
                    updated_at: null,
                },
            }),
        ]
        const update = applyToConversations(
            mine,
            {
                type: 'message.read',
                data: { conversation_id: 'a', reader_id: 'peer', message_id: 'm1' },
            },
            ME,
        )
        if (update.kind !== 'list') throw new Error(update.kind)
        expect(update.list[0]!.last_message_read).toBe(true)
    })

    it('reloads after my own read and on chat changes', () => {
        expect(
            applyToConversations(
                list,
                { type: 'message.read', data: { conversation_id: 'a', reader_id: ME } },
                ME,
            ).kind,
        ).toBe('invalidate')
        expect(
            applyToConversations(list, { type: 'conversation.deleted', data: {} }, ME).kind,
        ).toBe('invalidate')
    })

    it('ignores unrelated events', () => {
        expect(applyToConversations(list, { type: 'user.online', data: {} }, ME).kind).toBe(
            'ignore',
        )
    })
})
