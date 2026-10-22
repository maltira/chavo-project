import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import type { ConversationSummary } from '@/entities/conversation'
import { useGroupCard } from '@/entities/conversation'
import { useProfileCard } from '@/entities/user'

import { ChatActionsMenu } from './ChatActionsMenu'

const direct = {
    id: 'd1',
    conversation_type: 'direct',
    peer_id: 'peer',
    my_role: 'member',
    unread_count: 2,
    last_message: { id: 'm9' },
} as ConversationSummary

const group = {
    id: 'g1',
    conversation_type: 'group',
    name: 'Команда',
    peer_id: null,
    my_role: 'member',
    unread_count: 0,
    last_message: null,
} as ConversationSummary

const fetchMock = vi.fn()

function renderMenu(chat: ConversationSummary) {
    render(
        <QueryClientProvider client={new QueryClient()}>
            <MemoryRouter>
                <ChatActionsMenu chat={chat} position={{ x: 0, y: 0 }} onClose={vi.fn()} />
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('ChatActionsMenu', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        useProfileCard.setState({ userId: null })
        useGroupCard.setState({ target: null })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('a direct chat has info and read, but no leave', () => {
        renderMenu(direct)
        expect(screen.queryByRole('menuitem', { name: /Покинуть|Удалить/ })).toBeNull()

        fireEvent.click(screen.getByRole('menuitem', { name: 'Прочитать чат' }))
        expect(fetchMock).toHaveBeenCalledWith(
            '/api/conversations/d1/read',
            expect.objectContaining({ body: '{"message_id":"m9"}' }),
        )
        fireEvent.click(screen.getByRole('menuitem', { name: 'Сведения о чате' }))
        expect(useProfileCard.getState().userId).toBe('peer')
    })

    it('a member leaves the group after confirmation', async () => {
        renderMenu(group)
        expect(screen.queryByRole('menuitem', { name: 'Прочитать чат' })).toBeNull()
        fireEvent.click(screen.getByRole('menuitem', { name: 'Покинуть чат' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Покинуть' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/conversations/g1',
                expect.objectContaining({ method: 'DELETE' }),
            ),
        )
    })

    it('an admin deletes the group, info opens the group card', () => {
        renderMenu({ ...group, my_role: 'admin' })
        expect(screen.getByRole('menuitem', { name: 'Удалить чат' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('menuitem', { name: 'Сведения о чате' }))
        expect(useGroupCard.getState().target).toEqual({ kind: 'chat', id: 'g1' })
    })

    it('pins, archives and unmutes the chat', async () => {
        renderMenu({
            ...direct,
            muted_until: '9999-12-31T00:00:00Z',
            pinned_at: null,
            archived_at: null,
        })
        fireEvent.click(screen.getByRole('menuitem', { name: 'Закрепить чат' }))
        fireEvent.click(screen.getByRole('menuitem', { name: 'В архив' }))
        fireEvent.click(screen.getByRole('menuitem', { name: 'Включить звук' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3))
        const calls = fetchMock.mock.calls.map(([url, init]) => `${init.method} ${url}`)
        expect(calls).toEqual([
            'POST /api/conversations/d1/pin',
            'POST /api/conversations/d1/archive',
            'DELETE /api/conversations/d1/mute',
        ])
    })

    it('mutes for the picked duration', async () => {
        fetchMock.mockImplementation((url: string) =>
            Promise.resolve(
                new Response(
                    url === '/api/customization'
                        ? '{"status":{"icons":[],"durations":[]},"avatar_shapes":[],"mute_durations":["1h","8h","2d","forever"]}'
                        : '{"conversation_id":"d1","muted_until":"2026-01-01T08:00:00Z","archived_at":null,"pinned_at":null}',
                ),
            ),
        )
        renderMenu({ ...direct, muted_until: null })
        fireEvent.click(screen.getByRole('menuitem', { name: 'Без звука' }))
        const dialog = screen.getByRole('dialog', { name: 'Без звука' })
        fireEvent.click(await within(dialog).findByRole('button', { name: 'На 8 часов' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/conversations/d1/mute',
                expect.objectContaining({ method: 'POST', body: '{"duration":"8h"}' }),
            ),
        )
        await vi.waitFor(() =>
            expect(screen.queryByRole('dialog', { name: 'Без звука' })).toBeNull(),
        )
    })
})
