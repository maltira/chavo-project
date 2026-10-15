import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import type { Profile } from '@/entities/user'
import { useProfileCard, useRecentUsers } from '@/entities/user'

import { UserProfileModal } from './UserProfileModal'

const peer = {
    user_id: 'peer',
    username: 'vslxxx',
    display_name: 'Василиса Мамонтова',
    bio: 'Фотограф',
    online: false,
    last_seen_at: null,
    created_at: '2026-07-13T10:00:00Z',
    status: null,
    avatar_shape: null,
} as unknown as Profile

const fetchMock = vi.fn()

function renderModal() {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { ...peer, user_id: 'me', username: 'enemybye', display_name: 'Я' })
    client.setQueryData(['user', 'peer'], peer)
    client.setQueryData(['block-status', 'peer'], { blocked_by_me: false, blocked_by_them: false })
    return render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={['/']}>
                <Routes>
                    <Route path="/" element={<UserProfileModal />} />
                    <Route path="/chats/new/:userId" element={<p>новый чат</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('UserProfileModal', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        useProfileCard.setState({ userId: null })
        useRecentUsers.setState({ ids: [] })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('shows the profile and remembers it in recent', () => {
        renderModal()
        act(() => useProfileCard.getState().open('peer'))
        expect(screen.getByRole('heading', { name: 'Василиса Мамонтова' })).toBeInTheDocument()
        expect(screen.getByText('@vslxxx')).toBeInTheDocument()
        expect(screen.getByText('13 июля 2026 г.')).toBeInTheDocument()
        expect(screen.getByText('Фотограф')).toBeInTheDocument()
        expect(useRecentUsers.getState().ids).toEqual(['peer'])
    })

    it('opens a new chat when there is no direct chat yet', () => {
        renderModal()
        act(() => useProfileCard.getState().open('peer'))
        fireEvent.click(screen.getByRole('button', { name: 'Написать сообщение' }))
        expect(screen.getByText('новый чат')).toBeInTheDocument()
        expect(useProfileCard.getState().userId).toBeNull()
    })

    it('blocks after confirmation', async () => {
        renderModal()
        act(() => useProfileCard.getState().open('peer'))
        fireEvent.click(screen.getByRole('button', { name: 'Заблокировать' }))
        expect(fetchMock).not.toHaveBeenCalled()
        // подтверждение - отдельное окно поверх профиля
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Заблокировать' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/peer/block',
                expect.objectContaining({ method: 'POST' }),
            ),
        )
    })

    it('shows my own profile without actions', () => {
        renderModal()
        act(() => useProfileCard.getState().open('me', { remember: false }))
        expect(screen.getByRole('heading', { name: 'Я' })).toBeInTheDocument()
        expect(screen.queryByRole('button', { name: 'Написать сообщение' })).toBeNull()
        expect(useRecentUsers.getState().ids).toEqual([])
    })
})
