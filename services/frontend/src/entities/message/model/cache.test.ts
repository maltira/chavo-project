import { QueryClient } from '@tanstack/react-query'
import type { InfiniteData } from '@tanstack/react-query'

import { addMessage, applyMessageEvent, messagesKey, replaceMessage } from './cache'
import type { Message, MessagesPage } from './types'

const msg = (id: string, extra: Partial<Message> = {}): Message => ({
    id,
    conversation_id: 'c1',
    sender_id: 'a',
    content: id,
    reply_to_message_id: null,
    is_edited: false,
    is_deleted: false,
    created_at: '2026-10-14T10:00:00Z',
    updated_at: null,
    ...extra,
})

function setup(items: Message[]) {
    const client = new QueryClient()
    client.setQueryData<InfiniteData<MessagesPage, string | undefined>>(messagesKey('c1'), {
        pages: [{ items, limit: 50, next_before: null }],
        pageParams: [undefined],
    })
    const ids = () =>
        client
            .getQueryData<InfiniteData<MessagesPage>>(messagesKey('c1'))!
            .pages.flatMap((p) => p.items.map((m) => m.id))
    return { client, ids }
}

describe('message cache', () => {
    it('puts a new message first and does not duplicate it', () => {
        const { client, ids } = setup([msg('m1')])
        addMessage(client, msg('m2'))
        addMessage(client, msg('m2'))
        expect(ids()).toEqual(['m2', 'm1'])
    })

    it('replaces the temporary message with the saved one', () => {
        const { client, ids } = setup([msg('temp-1', { pending: true }), msg('m1')])
        replaceMessage(client, 'temp-1', msg('m2'))
        expect(ids()).toEqual(['m2', 'm1'])
    })

    it('drops the temporary message when the event came first', () => {
        const { client, ids } = setup([msg('m2'), msg('temp-1', { pending: true }), msg('m1')])
        replaceMessage(client, 'temp-1', msg('m2'))
        expect(ids()).toEqual(['m2', 'm1'])
    })

    it('keeps the row key of my message in both orders of reply and event', () => {
        const items = (client: QueryClient) =>
            client.getQueryData<InfiniteData<MessagesPage>>(messagesKey('c1'))!.pages[0]!.items

        // ответ сервера, потом событие о том же сообщении
        const first = setup([msg('temp-1', { pending: true })])
        replaceMessage(first.client, 'temp-1', msg('m2'))
        addMessage(first.client, msg('m2', { content: 'из события' }))
        expect(items(first.client)[0]).toMatchObject({
            id: 'm2',
            content: 'из события',
            local_key: 'temp-1',
        })

        // событие раньше ответа: ключ временного переходит к пришедшему
        const second = setup([msg('m2'), msg('temp-1', { pending: true })])
        replaceMessage(second.client, 'temp-1', msg('m2'))
        expect(items(second.client)).toEqual([
            expect.objectContaining({ id: 'm2', local_key: 'temp-1' }),
        ])
    })

    it('applies created, updated and deleted events', () => {
        const { client, ids } = setup([msg('m1')])
        applyMessageEvent(client, {
            type: 'message.created',
            data: {
                conversation_id: 'c1',
                message_id: 'm2',
                sender_id: 'b',
                content: 'привет',
                created_at: '2026-10-14T11:00:00Z',
            },
        })
        applyMessageEvent(client, {
            type: 'message.updated',
            data: { conversation_id: 'c1', message_id: 'm2', content: 'исправлено' },
        })
        applyMessageEvent(client, {
            type: 'message.deleted',
            data: { conversation_id: 'c1', message_id: 'm1' },
        })
        expect(ids()).toEqual(['m2', 'm1'])
        const pages = client.getQueryData<InfiniteData<MessagesPage>>(messagesKey('c1'))!.pages
        expect(pages[0]!.items[0]).toMatchObject({ content: 'исправлено', is_edited: true })
        expect(pages[0]!.items[1]).toMatchObject({ content: null, is_deleted: true })
    })
})
