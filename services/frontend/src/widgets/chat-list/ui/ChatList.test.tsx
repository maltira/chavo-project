import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
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
    muted_until: null,
    archived_at: null,
    pinned_at: null,
    is_saved: false,
    slug: null,
}

function renderList(
    chats: ConversationSummary[] | undefined,
    compact = false,
    archived: ConversationSummary[] = [],
) {
    // данные заданы заранее и свежие: сетевых запросов нет
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { user_id: 'me' })
    if (chats) {
        client.setQueryData(['conversations'], { pages: [chats], pageParams: [0] })
    }
    client.setQueryData(['conversations', 'archived'], { pages: [archived], pageParams: [0] })
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

    it('marks pinned and muted chats, a muted counter is grey', () => {
        renderList([
            { ...base, id: 'p', name: 'Закреплённый', pinned_at: '2026-10-14T10:00:00Z' },
            {
                ...base,
                id: 'm',
                name: 'Тихий',
                unread_count: 4,
                muted_until: '9999-12-31T00:00:00Z',
            },
        ])
        expect(
            within(screen.getByRole('link', { name: /Закреплённый/ })).getByRole('img', {
                name: 'Закреплён',
            }),
        ).toBeInTheDocument()
        const quiet = screen.getByRole('link', { name: /Тихий/ })
        expect(within(quiet).getByRole('img', { name: 'Без звука' })).toBeInTheDocument()
        expect(within(quiet).getByText('4').className).toMatch(/muted/)
    })

    it('opens the archive from the row above the list and goes back', () => {
        renderList([base], false, [
            { ...base, id: 'a1', name: 'Старый', unread_count: 2 },
            { ...base, id: 'a2', name: 'Ещё старее' },
        ])
        const row = screen.getByRole('button', { name: /Архив/ })
        expect(within(row).getByText('2 чата')).toBeInTheDocument()
        expect(screen.queryByText('Старый')).toBeNull()

        fireEvent.click(row)
        expect(screen.getByText('Старый')).toBeInTheDocument()
        expect(screen.queryByRole('link', { name: /Команда/ })).toBeNull()

        fireEvent.click(screen.getByRole('button', { name: 'Назад к чатам' }))
        expect(screen.getByRole('link', { name: /Команда/ })).toBeInTheDocument()
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

    it('shows «Избранное» with a star and without read marks', () => {
        renderList([
            {
                ...base,
                conversation_type: 'direct',
                name: null,
                peer_id: null,
                is_saved: true,
                last_message_read: true,
            },
        ])
        const link = screen.getByRole('link', { name: /Избранное/ })
        // в «Избранном» все сообщения свои - без «Вы:»
        expect(within(link).getByText('Привет всем')).toBeInTheDocument()
        expect(link.querySelector('img')).toBeNull()
    })
})
