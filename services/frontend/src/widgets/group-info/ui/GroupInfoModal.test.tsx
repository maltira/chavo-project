import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { useGroupCard } from '@/entities/conversation'
import type { ConversationSummary, GroupPreview } from '@/entities/conversation'

import { GroupInfoModal } from './GroupInfoModal'

// чтение и кодирование фото в jsdom недоступны (нет canvas): подменяются
const { loadImage, encodeSquare } = vi.hoisted(() => ({
    loadImage: vi.fn(),
    encodeSquare: vi.fn(),
}))
vi.mock('@/shared/lib', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/shared/lib')>()),
    loadImage,
    encodeSquare,
}))

const chat: ConversationSummary = {
    id: 'g1',
    conversation_type: 'group',
    visibility: 'private',
    name: 'Крутые перцы',
    description: 'Обсуждаем всё подряд',
    avatar_url: null,
    last_message_at: '2026-07-13T10:00:00Z',
    created_at: '2026-07-13T10:00:00Z',
    updated_at: '2026-07-13T10:00:00Z',
    status: null,
    avatar_shape: null,
    slug: null,
    my_role: 'admin',
    peer_id: null,
    last_message: null,
    unread_count: 0,
    last_message_read: false,
    members_count: 2,
    muted_until: null,
    archived_at: null,
    pinned_at: null,
    is_saved: false,
    invite_code: 'abc123def456',
}

const preview: GroupPreview = {
    id: 'g2',
    name: 'Книжный клуб',
    description: 'Читаем по книге в месяц',
    avatar_url: null,
    members_count: 4,
    is_member: false,
    visibility: 'public',
    slug: 'book_club',
    request_status: null,
    status: null,
    avatar_shape: null,
}

const profile = (id: string, name: string) => ({
    user_id: id,
    username: name.toLowerCase(),
    display_name: name,
    last_seen_at: null,
    online: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
})

const json = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let current = chat
const fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET' && url === '/api/conversations/g1') {
        return Promise.resolve(json(current))
    }
    if (method === 'GET' && url.startsWith('/api/conversations/g1/members')) {
        return Promise.resolve(
            json({
                items: [
                    { conversation_id: 'g1', user_id: 'me', role: 'admin', joined_at: '' },
                    { conversation_id: 'g1', user_id: 'u2', role: 'member', joined_at: '' },
                ],
                limit: 50,
                offset: 0,
            }),
        )
    }
    if (method === 'GET' && url.startsWith('/api/conversations/g1/join-requests')) {
        return Promise.resolve(
            json({
                items: [{ id: 'r1', conversation_id: 'g1', user_id: 'u3', status: 'pending' }],
                limit: 50,
                offset: 0,
            }),
        )
    }
    if (url.endsWith('/invite-link/regenerate')) {
        return Promise.resolve(json({ invite_token: 'newcode00000' }))
    }
    return Promise.resolve(json({ message: 'ok' }))
})

function renderModal(summary: ConversationSummary = chat) {
    current = summary
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['conversation', 'g1'], summary)
    client.setQueryData(['me'], profile('me', 'Я'))
    client.setQueryData(['user', 'me'], profile('me', 'Я'))
    client.setQueryData(['user', 'u2'], profile('u2', 'Герман'))
    client.setQueryData(['user', 'u3'], profile('u3', 'Давыд'))
    render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={['/']}>
                <Routes>
                    <Route path="/" element={<GroupInfoModal />} />
                    <Route path="/chats/:id" element={<p>чат группы</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

const calls = (method: string, url: string) =>
    fetchMock.mock.calls.filter(([u, init]) => u === url && (init?.method ?? 'GET') === method)

describe('GroupInfoModal', () => {
    beforeEach(() => {
        fetchMock.mockClear()
        vi.stubGlobal('fetch', fetchMock)
        Object.assign(navigator, { clipboard: { writeText: vi.fn(() => Promise.resolve()) } })
        useGroupCard.setState({ target: null })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('joins an open group from the preview and opens its chat', async () => {
        renderModal()
        act(() => useGroupCard.getState().openPreview(preview))
        expect(screen.getByRole('heading', { name: 'Книжный клуб' })).toBeInTheDocument()
        expect(screen.getByText('4 участника')).toBeInTheDocument()
        expect(screen.getByText(/\/chat\/book_club$/)).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Вступить' }))
        expect(await screen.findByText('чат группы')).toBeInTheDocument()
        expect(calls('POST', '/api/conversations/g2/join')).toHaveLength(1)
        expect(useGroupCard.getState().target).toBeNull()
    })

    it('sends a join request to a private group with the link code', async () => {
        renderModal()
        act(() =>
            useGroupCard
                .getState()
                .openPreview({ ...preview, visibility: 'private', slug: null, code: 'secret' }),
        )
        fireEvent.click(screen.getByRole('button', { name: 'Отправить запрос' }))
        expect(await screen.findByRole('button', { name: 'Запрос отправлен' })).toBeDisabled()
        const init = calls('POST', '/api/conversations/g2/join-requests')[0]![1]
        expect(JSON.parse(init!.body as string)).toEqual({ invite_token: 'secret' })
    })

    it('a member of the group goes straight to the chat card', () => {
        renderModal({ ...chat, my_role: 'member', invite_code: null })
        act(() => useGroupCard.getState().openPreview({ ...preview, id: 'g1', is_member: true }))
        expect(screen.getByRole('heading', { name: 'Крутые перцы' })).toBeInTheDocument()
        // закрытую ссылку и настройки участник не видит
        expect(screen.queryByText('Ссылка на чат')).toBeNull()
        expect(screen.queryByRole('button', { name: 'Настройки группы' })).toBeNull()
        expect(screen.getByRole('button', { name: 'Участники' })).toBeInTheDocument()
    })

    it('an admin copies and resets the private link', async () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        const link = screen.getByRole('button', { name: 'Скопировать ссылку на чат' })
        expect(link).toHaveTextContent('/chat/abc123def456')
        fireEvent.click(link)
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
            `${window.location.origin}/chat/abc123def456`,
        )

        fireEvent.click(screen.getByRole('button', { name: 'Сбросить ссылку' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Сбросить' }))
        await waitFor(() =>
            expect(
                screen.getByRole('button', { name: 'Скопировать ссылку на чат' }),
            ).toHaveTextContent('/chat/newcode00000'),
        )
    })

    it('an admin changes roles and bans from the member menu', async () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Участники' }))
        expect(screen.getByText('Все участники: 2')).toBeInTheDocument()
        const row = (await screen.findByRole('button', { name: 'Герман' })).parentElement!
        expect(screen.getByText('Админ')).toBeInTheDocument()

        fireEvent.contextMenu(row, { clientX: 10, clientY: 10 })
        fireEvent.click(screen.getByRole('menuitem', { name: 'Сделать админом' }))
        await waitFor(() =>
            expect(calls('POST', '/api/conversations/g1/members/u2')).toHaveLength(1),
        )
        const init = calls('POST', '/api/conversations/g1/members/u2')[0]![1]
        expect(JSON.parse(init!.body as string)).toEqual({ role: 'admin' })

        fireEvent.contextMenu(row, { clientX: 10, clientY: 10 })
        fireEvent.click(screen.getByRole('menuitem', { name: 'Исключить и заблокировать' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Заблокировать' }))
        await waitFor(() =>
            expect(calls('DELETE', '/api/conversations/g1/members/u2?ban=true')).toHaveLength(1),
        )
    })

    it('has no member menu on my own row', async () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Участники' }))
        const mine = (await screen.findByRole('button', { name: 'Я' })).parentElement!
        fireEvent.contextMenu(mine, { clientX: 10, clientY: 10 })
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('an admin accepts a join request', async () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Участники' }))
        fireEvent.click(await screen.findByRole('radio', { name: 'Заявки (1)' }))
        expect(await screen.findByRole('button', { name: 'Давыд' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Принять' }))
        await waitFor(() =>
            expect(calls('POST', '/api/conversations/g1/join-requests/r1/approve')).toHaveLength(1),
        )
    })

    it('picks the group shape and status on screens with a back arrow', () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Настройки группы' }))

        fireEvent.click(screen.getByRole('button', { name: 'Изменить фигуру за аватаром' }))
        expect(screen.getAllByRole('dialog')).toHaveLength(1)
        expect(screen.getByRole('dialog', { name: 'Фигура' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(screen.getByRole('dialog', { name: /Настройки группы/ })).toBeInTheDocument()
    })

    it('an admin uploads and removes the group photo from the settings', async () => {
        loadImage.mockResolvedValue({ source: {}, width: 300, height: 300 })
        encodeSquare.mockResolvedValue(new Blob(['jpeg'], { type: 'image/jpeg' }))
        renderModal({ ...chat, avatar_url: '/media/avatars/old.jpg' })
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Настройки группы' }))

        const file = new File([new Uint8Array(10)], 'g.png', { type: 'image/png' })
        fireEvent.change(screen.getByTestId('photo-input'), { target: { files: [file] } })
        const save = await screen.findByRole('button', { name: 'Сохранить' })
        await waitFor(() => expect(save).toBeEnabled())
        await act(async () => fireEvent.click(save))
        await waitFor(() => expect(calls('POST', '/api/conversations/g1/avatar')).toHaveLength(1))
        expect(calls('POST', '/api/conversations/g1/avatar')[0]![1]!.body).toBeInstanceOf(FormData)

        fireEvent.click(screen.getByRole('button', { name: 'Удалить фото' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Удалить' }))
        await waitFor(() => expect(calls('DELETE', '/api/conversations/g1/avatar')).toHaveLength(1))
    })

    it('switching to open requires a link and saves both', async () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Настройки группы' }))
        fireEvent.click(screen.getByRole('radio', { name: 'Открытый' }))
        expect(screen.getByText(/Прежняя ссылка на чат перестанет работать/)).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }))
        expect(screen.getByText(/От 3 до 32 символов/)).toBeInTheDocument()
        expect(calls('PATCH', '/api/conversations/g1')).toHaveLength(0)

        fireEvent.change(screen.getByLabelText('Ссылка на чат'), { target: { value: 'peppers' } })
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }))
        await waitFor(() => expect(calls('PATCH', '/api/conversations/g1')).toHaveLength(1))
        const init = calls('PATCH', '/api/conversations/g1')[0]![1]
        expect(JSON.parse(init!.body as string)).toEqual({ visibility: 'public', slug: 'peppers' })
        // после сохранения - снова сведения
        expect(await screen.findByRole('button', { name: 'Участники' })).toBeInTheDocument()
    })
})

describe('GroupInfoModal reopening', () => {
    beforeEach(() => {
        fetchMock.mockClear()
        vi.stubGlobal('fetch', fetchMock)
        useGroupCard.setState({ target: null })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('starts from the group info after it was closed on another screen', () => {
        renderModal()
        act(() => useGroupCard.getState().openChat('g1'))
        fireEvent.click(screen.getByRole('button', { name: 'Участники' }))
        expect(screen.getByRole('dialog', { name: 'Участники' })).toBeInTheDocument()

        act(() => useGroupCard.getState().close())
        act(() => useGroupCard.getState().openChat('g1'))
        expect(screen.getByRole('dialog', { name: 'Группа: Крутые перцы' })).toBeInTheDocument()
        expect(screen.queryByRole('dialog', { name: 'Участники' })).toBeNull()
    })
})
