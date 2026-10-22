import { applyChatSettings, applyToConversations } from './events'
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
        members_count: 2,
        muted_until: null,
        archived_at: null,
        pinned_at: null,
        is_saved: false,
        slug: null,
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

    it('lifts the chat with a system message but does not count it as unread', () => {
        const update = applyToConversations(
            list,
            {
                type: 'message.created',
                data: {
                    conversation_id: 'b',
                    message_id: 's1',
                    sender_id: 'peer',
                    created_at: '2026-10-14T11:00:00Z',
                    kind: 'system',
                    system_event: 'joined',
                    target_id: 'peer',
                },
            },
            ME,
        )
        if (update.kind !== 'list') throw new Error('list expected')
        expect(update.list.map((c) => c.id)).toEqual(['b', 'a'])
        expect(update.list[0]).toMatchObject({
            unread_count: 0,
            last_message: { id: 's1', kind: 'system', system_event: 'joined', content: null },
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

describe('applyChatSettings', () => {
    const settings = (data: Record<string, unknown>) => ({
        type: 'conversation.settings.updated',
        data: {
            conversation_id: 'b',
            muted_until: null,
            archived_at: null,
            pinned_at: null,
            ...data,
        },
    })
    const main = [chat('a', '2026-10-14T10:00:00Z'), chat('b', '2026-10-14T09:00:00Z')]

    it('puts a pinned chat above fresher ones', () => {
        const update = applyChatSettings(main, [], settings({ pinned_at: '2026-10-14T12:00:00Z' }))
        expect(update.main?.map((c) => c.id)).toEqual(['b', 'a'])
        expect(update.archived).toEqual([])
    })

    it('newer pin goes higher, times with different fraction lengths compare as dates', () => {
        const pinned = [
            chat('a', '2026-10-14T10:00:00Z', { pinned_at: '2026-10-14T12:00:00.2Z' }),
            chat('b', '2026-10-14T09:00:00Z'),
        ]
        const update = applyChatSettings(
            pinned,
            [],
            settings({ pinned_at: '2026-10-14T12:00:00.21Z' }),
        )
        expect(update.main?.map((c) => c.id)).toEqual(['b', 'a'])
    })

    it('moves the chat to the archive and back', () => {
        const toArchive = applyChatSettings(
            main,
            [],
            settings({ archived_at: '2026-10-14T12:00:00Z' }),
        )
        expect(toArchive.main?.map((c) => c.id)).toEqual(['a'])
        expect(toArchive.archived?.map((c) => c.id)).toEqual(['b'])

        const back = applyChatSettings(toArchive.main, toArchive.archived, settings({}))
        expect(back.main?.map((c) => c.id)).toEqual(['a', 'b'])
        expect(back.archived).toEqual([])
    })

    it('updates mute in place and asks to reload for an unknown chat', () => {
        const muted = applyChatSettings(
            main,
            undefined,
            settings({ muted_until: '9999-12-31T00:00:00Z' }),
        )
        expect(muted.main?.find((c) => c.id === 'b')?.muted_until).toBe('9999-12-31T00:00:00Z')
        expect(muted.archived).toBeUndefined()
        expect(
            applyChatSettings(main, [], { ...settings({}), data: { conversation_id: 'x' } }).reload,
        ).toBe(true)
    })
})
