import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import type { ConversationSummary } from '@/entities/conversation'
import { useNotifications } from '@/shared/lib'

import { ChatList } from './ChatList'

const base: ConversationSummary = {
    id: 'c1',
    conversation_type: 'group',
    visibility: 'public',
    name: 'Команда',
    description: null,
    avatar_url: null,
    last_message_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    status: null,
    avatar_shape: null,
    my_role: 'member',
    peer_id: null,
    last_message: {
        id: 'm1',
        conversation_id: 'c1',
        sender_id: 'me',
        content: 'Привет всем',
        reply_to_message_id: null,
        is_edited: false,
        is_deleted: false,
        created_at: new Date().toISOString(),
        updated_at: null,
    },
    unread_count: 0,
    last_message_read: true,
    members_count: 2,
    slug: null,
}

function renderList(chats: ConversationSummary[] | undefined, compact = false) {
    // данные заданы заранее и свежие: сетевых запросов нет
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { user_id: 'me' })
    if (chats) {
        client.setQueryData(['conversations'], { pages: [chats], pageParams: [0] })
    }
    return render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={['/chats/c1']}>
                <ChatList compact={compact} />
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('ChatList', () => {
    beforeEach(() => {
        // запросы списка и профилей не должны уходить в сеть
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        useNotifications.setState({ items: [] })
    })

    it('shows the chat with my last message and marks the open chat', () => {
        renderList([base])
        const link = screen.getByRole('link', { name: /Команда/ })
        expect(link).toHaveAttribute('href', '/chats/c1')
        expect(link).toHaveAttribute('aria-current', 'page')
        expect(screen.getByText('Вы: Привет всем')).toBeInTheDocument()
    })

    it('shows the unread counter, capped at 99+', () => {
        renderList([
            {
                ...base,
                unread_count: 150,
                last_message: {
                    ...base.last_message!,
                    sender_id: 'peer',
                    is_deleted: true,
                    content: null,
                },
            },
        ])
        expect(screen.getByText('99+')).toBeInTheDocument()
        expect(screen.getByText('Сообщение удалено')).toBeInTheDocument()
    })

    it('shows only avatars with the counter in compact mode', () => {
        renderList([{ ...base, unread_count: 3 }], true)
        expect(screen.getByRole('link', { name: 'Команда' })).toBeInTheDocument()
        expect(screen.queryByText('Вы: Привет всем')).toBeNull()
        expect(screen.getByText('3')).toBeInTheDocument()
    })

    it('shows the empty state', () => {
        renderList([])
        expect(screen.getByText('Здесь пока что пусто')).toBeInTheDocument()
    })

    it('notifies and offers a retry when the next page fails, keeping loaded chats', async () => {
        // конец списка сразу «на виду»: наблюдатель просит следующую страницу
        class VisibleObserver {
            constructor(private callback: (entries: { isIntersecting: boolean }[]) => void) {}
            observe() {
                this.callback([{ isIntersecting: true }])
            }
            disconnect() {}
        }
        vi.stubGlobal('IntersectionObserver', VisibleObserver)
        const fetchMock = vi
            .fn()
            .mockResolvedValue(new Response('{"error":"boom"}', { status: 500 }))
        vi.stubGlobal('fetch', fetchMock)

        const full = Array.from({ length: 30 }, (_, i) => ({
            ...base,
            id: `c${i}`,
            name: `Чат ${i}`,
        }))
        renderList(full)

        expect(await screen.findByRole('button', { name: 'Повторить' })).toBeInTheDocument()
        expect(screen.getByText('Чат 0')).toBeInTheDocument()
        expect(useNotifications.getState().items.map((n) => n.id)).toContain('chats-load')
        // без цикла повторов: одна неудачная попытка
        expect(
            fetchMock.mock.calls.filter(([url]) => String(url).includes('offset=30')),
        ).toHaveLength(1)
    })

    it('shows the skeleton while loading', () => {
        renderList(undefined)
        expect(screen.getByLabelText('Загрузка чатов')).toHaveAttribute('aria-busy', 'true')
    })
})
