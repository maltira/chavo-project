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
})
