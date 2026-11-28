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
    return {
        client,
        ...render(
            <QueryClientProvider client={client}>
                <MemoryRouter initialEntries={['/']}>
                    <Routes>
                        <Route path="/" element={<UserProfileModal />} />
                        <Route path="/chats/new/:userId" element={<p>новый чат</p>} />
                    </Routes>
                </MemoryRouter>
            </QueryClientProvider>,
        ),
    }
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

    it('picks the appearance from the edit screen with a back arrow, not in another window', () => {
        renderModal()
        act(() => useProfileCard.getState().open('me'))
        expect(screen.queryByRole('button', { name: 'Изменить оформление' })).toBeNull()

        fireEvent.click(screen.getByRole('button', { name: 'Редактировать' }))
        fireEvent.click(screen.getByRole('button', { name: 'Изменить оформление' }))

        // экран оформления заменяет окно профиля, а не открывается поверх него
        expect(screen.getAllByRole('dialog')).toHaveLength(1)
        expect(screen.getByRole('dialog', { name: 'Оформление' })).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(screen.getAllByRole('dialog')).toHaveLength(1)
        expect(screen.getByRole('dialog', { name: /Профиль: Я/ })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Изменить оформление' })).toBeInTheDocument()
    })

    it('renders profile cover halo when profile_cover is set', () => {
        const { client } = renderModal()
        client.setQueryData(['user', 'peer'], {
            ...peer,
            profile_cover: { color: '#4F46E5', pattern: 'star' },
        })
        act(() => useProfileCard.getState().open('peer'))
        const dialog = screen.getByRole('dialog', { name: /Профиль: Василиса/ })
        const cover = dialog.querySelector('[data-cover-color="#4F46E5"]')
        expect(cover).toBeInTheDocument()
        expect(cover).toHaveAttribute('data-cover-pattern', 'star')
    })

    it('shows my own profile without actions', () => {
        renderModal()
        act(() => useProfileCard.getState().open('me', { remember: false }))
        expect(screen.getByRole('heading', { name: 'Я' })).toBeInTheDocument()
        expect(screen.queryByRole('button', { name: 'Написать сообщение' })).toBeNull()
        expect(useRecentUsers.getState().ids).toEqual([])
    })

    const photo = (c: string) => `/media/avatars/${c.repeat(32)}.jpg`
    const history = (owner: string) =>
        ['a', 'b', 'c'].map((c, i) => ({
            id: `${owner}-${c}`,
            avatar_url: photo(c),
            created_at: '2026-10-01T10:00:00Z',
            is_current: i === 0,
        }))
    // GET истории отдаёт три фото, перечитанный профиль - прежний, остальное - «ок»
    const respond = (owner: string) =>
        fetchMock.mockImplementation((url: string) =>
            Promise.resolve(
                new Response(
                    url === `/api/users/${owner}/avatars`
                        ? JSON.stringify({ items: history(owner) })
                        : url === '/api/users/me'
                          ? JSON.stringify({
                                ...peer,
                                user_id: 'me',
                                display_name: 'Я',
                                avatar_url: photo('a'),
                            })
                          : '{"message":"ok"}',
                ),
            ),
        )

    it('opens the photo of another person full-screen and pages through their history', async () => {
        respond('peer')
        const { client } = renderModal()
        client.setQueryData(['user', 'peer'], { ...peer, avatar_url: photo('a') })
        act(() => useProfileCard.getState().open('peer'))
        fireEvent.click(screen.getByRole('button', { name: 'Открыть фото' }))
        const viewer = screen.getByRole('dialog', { name: 'Фото: Василиса Мамонтова' })
        // на весь экран - крупная версия фото; историю не видно, пока она не загрузилась
        expect(viewer.querySelector('img')).toHaveAttribute(
            'src',
            `/media/avatars/${'a'.repeat(32)}_l.jpg`,
        )
        expect(await within(viewer).findByText('1 из 3')).toBeInTheDocument()
        expect(fetchMock).toHaveBeenCalledWith('/api/users/peer/avatars', expect.anything())

        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        expect(viewer.querySelector('img')).toHaveAttribute(
            'src',
            `/media/avatars/${'b'.repeat(32)}_l.jpg`,
        )
        // чужой профиль: управлять историей нельзя
        expect(within(viewer).getByRole('button', { name: 'Скачать' })).toBeInTheDocument()
        expect(within(viewer).queryByRole('button', { name: 'Удалить фото' })).toBeNull()
        expect(within(viewer).queryByRole('button', { name: 'Сделать главной' })).toBeNull()

        // Esc закрывает только просмотр, профиль остаётся
        fireEvent.keyDown(document, { key: 'Escape' })
        expect(screen.queryByRole('dialog', { name: /^Фото/ })).toBeNull()
        expect(screen.getByRole('dialog', { name: /Профиль: Василиса/ })).toBeInTheDocument()
    })

    it('my own history: make a photo the main one and delete exactly the shown one', async () => {
        respond('me')
        const { client } = renderModal()
        client.setQueryData(['me'], {
            ...peer,
            user_id: 'me',
            display_name: 'Я',
            avatar_url: photo('a'),
        })
        act(() => useProfileCard.getState().open('me', { remember: false }))
        // в просмотре профиля кнопки смены оформления нет - она перенесена в «Редактировать»
        expect(screen.queryByRole('button', { name: 'Изменить оформление' })).toBeNull()

        fireEvent.click(screen.getByRole('button', { name: 'Открыть фото' }))
        const viewer = screen.getByRole('dialog', { name: 'Фото: Я' })
        await within(viewer).findByText('1 из 3')
        fireEvent.keyDown(document, { key: 'ArrowLeft' })

        fireEvent.click(within(viewer).getByRole('button', { name: 'Сделать главной' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/me/avatars/me-b/select',
                expect.objectContaining({ method: 'POST' }),
            ),
        )

        fireEvent.click(within(viewer).getByRole('button', { name: 'Удалить фото' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Удалить' }))
        // удаляется именно показанное фото из истории, а не текущее
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/me/avatars/me-b',
                expect.objectContaining({ method: 'DELETE' }),
            ),
        )
    })

    it('deletes the current photo through the plain endpoint while the history is loading', async () => {
        // история не успела загрузиться: показано только текущее фото, оно и удаляется
        fetchMock.mockImplementation((url: string) =>
            url.endsWith('/avatars')
                ? new Promise(() => {})
                : Promise.resolve(new Response('{"message":"ok"}')),
        )
        const { client } = renderModal()
        client.setQueryData(['me'], {
            ...peer,
            user_id: 'me',
            display_name: 'Я',
            avatar_url: photo('a'),
        })
        act(() => useProfileCard.getState().open('me', { remember: false }))
        fireEvent.click(screen.getByRole('button', { name: 'Открыть фото' }))
        fireEvent.click(screen.getByRole('button', { name: 'Удалить фото' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Удалить' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/me/avatar',
                expect.objectContaining({ method: 'DELETE' }),
            ),
        )
    })
})
