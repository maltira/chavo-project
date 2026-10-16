import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'

import type { ConversationSummary } from '@/entities/conversation'
import type { Message } from '@/entities/message'

import { MessageList } from './MessageList'

const now = Date.now()
// свежие первыми; at - сколько минут назад
const msg = (id: string, sender: string, at: number) =>
    ({
        id,
        conversation_id: 'g1',
        sender_id: sender,
        content: id,
        reply_to_message_id: null,
        is_edited: false,
        is_deleted: false,
        created_at: new Date(now - at * 60_000).toISOString(),
        updated_at: null,
    }) as Message

function renderList(conversation_type: 'group' | 'direct') {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } })
    client.setQueryData(['user', 'a'], { user_id: 'a', display_name: 'Давыд' })
    client.setQueryData(['user', 'b'], { user_id: 'b', display_name: 'Герман' })
    render(
        <QueryClientProvider client={client}>
            <MessageList
                chat={{ id: 'g1', conversation_type, my_role: 'member' } as ConversationSummary}
                // снизу вверх: a (2 подряд), моё, b, a
                messages={[
                    msg('a3', 'a', 1),
                    msg('a2', 'a', 2),
                    msg('mine', 'me', 3),
                    msg('b1', 'b', 4),
                    msg('a1', 'a', 5),
                ]}
                myId="me"
                readBoundary={undefined}
                hasOlder={false}
                loadingOlder={false}
                loadOlder={vi.fn()}
                firstUnreadId={null}
                findMessage={vi.fn()}
                onSeen={vi.fn()}
                onReply={vi.fn()}
                onEdit={vi.fn()}
                onDelete={vi.fn()}
            />
        </QueryClientProvider>,
    )
}

describe('MessageList avatars', () => {
    beforeEach(() => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('shows the author avatar only under the last message of a series in a group', () => {
        renderList('group')
        // серия a3+a2 - один аватар, b1 и a1 - по одному; у своего сообщения аватара нет
        expect(screen.getAllByRole('button', { name: 'Профиль: Давыд' })).toHaveLength(2)
        expect(screen.getAllByRole('button', { name: 'Профиль: Герман' })).toHaveLength(1)
        const series = document.querySelector('[data-message-id="a3"]')!
        expect(series.querySelector('button[aria-label^="Профиль"]')).not.toBeNull()
        const above = document.querySelector('[data-message-id="a2"]')!
        expect(above.querySelector('button[aria-label^="Профиль"]')).toBeNull()
    })

    it('pulls a message closer to the previous one of the same author', () => {
        renderList('group')
        // a3 идёт сразу за a2 того же автора, a2 - после моего сообщения
        expect(document.querySelector('[data-message-id="a3"]')!.className).toMatch(/continued/)
        expect(document.querySelector('[data-message-id="a2"]')!.className).not.toMatch(/continued/)
    })

    it('has no avatars in a direct chat', () => {
        renderList('direct')
        expect(screen.queryByRole('button', { name: /^Профиль/ })).toBeNull()
    })
})
