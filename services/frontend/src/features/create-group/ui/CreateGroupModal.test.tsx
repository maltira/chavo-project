import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { CreateGroupModal } from './CreateGroupModal'

const fetchMock = vi.fn()

function renderModal() {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['conversations'], {
        pages: [
            [
                { id: 'd1', conversation_type: 'direct', peer_id: 'u1' },
                { id: 'd2', conversation_type: 'direct', peer_id: 'u2' },
                { id: 'g1', conversation_type: 'group', peer_id: null },
            ],
        ],
        pageParams: [0],
    })
    client.setQueryData(['user', 'u1'], { user_id: 'u1', display_name: 'Давыд', username: 'davyd' })
    client.setQueryData(['user', 'u2'], {
        user_id: 'u2',
        display_name: 'Герман',
        username: 'german',
    })
    render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={['/']}>
                <Routes>
                    <Route path="/" element={<CreateGroupModal open onClose={vi.fn()} />} />
                    <Route path="/chats/:id" element={<p>новая группа</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('CreateGroupModal', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response(JSON.stringify({ id: 'g-new' }), { status: 201 }))
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('creates a private group with the chosen people and opens it', async () => {
        renderModal()
        const next = screen.getByRole('button', { name: 'Продолжить' })
        expect(next).toBeDisabled()

        fireEvent.change(screen.getByLabelText('Название'), { target: { value: '  Команда  ' } })
        fireEvent.click(screen.getByRole('radio', { name: 'Закрытый' }))
        expect(screen.getByText(/только по ссылке-приглашению/)).toBeInTheDocument()
        fireEvent.change(screen.getByLabelText('Описание'), {
            target: { value: 'Обсуждаем релиз' },
        })
        fireEvent.click(next)

        // второй шаг: только собеседники из личных чатов
        expect(screen.getByRole('dialog', { name: 'Участники' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Добавить Герман' }))
        expect(screen.getByText('Выбрано участников: 1')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Создать чат' }))

        expect(await screen.findByText('новая группа')).toBeInTheDocument()
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/conversations')
        expect(JSON.parse(init.body as string)).toEqual({
            name: 'Команда',
            description: 'Обсуждаем релиз',
            visibility: 'private',
            member_ids: ['u2'],
        })
    })

    it('a public group needs a valid chat link and sends it', async () => {
        renderModal()
        fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Клуб' } })
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
        // без ссылки дальше не пускает
        expect(screen.getByText(/От 3 до 32 символов/)).toBeInTheDocument()
        expect(screen.queryByRole('dialog', { name: 'Участники' })).toBeNull()

        // недопустимые символы не вводятся
        fireEvent.change(screen.getByLabelText('Ссылка на чат'), {
            target: { value: 'book club!' },
        })
        expect(screen.getByLabelText('Ссылка на чат')).toHaveValue('bookclub')
        expect(screen.getByText(/\/chat\/bookclub$/)).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
        fireEvent.click(screen.getByRole('button', { name: 'Создать чат' }))
        expect(await screen.findByText('новая группа')).toBeInTheDocument()
        const [, init] = fetchMock.mock.calls[0]!
        expect(JSON.parse(init.body as string)).toMatchObject({
            visibility: 'public',
            slug: 'bookclub',
        })
    })

    it('returns to the link field when the link is taken', async () => {
        fetchMock.mockResolvedValue(
            new Response(
                JSON.stringify({ error: 'Эта ссылка на чат уже занята', reason: 'SLUG_TAKEN' }),
                { status: 409 },
            ),
        )
        renderModal()
        fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Клуб' } })
        fireEvent.change(screen.getByLabelText('Ссылка на чат'), { target: { value: 'club' } })
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
        fireEvent.click(screen.getByRole('button', { name: 'Создать чат' }))
        expect(await screen.findByText('Эта ссылка на чат уже занята')).toBeInTheDocument()
        expect(screen.getByLabelText('Ссылка на чат')).toHaveAttribute('aria-invalid', 'true')
    })

    it('goes back to the details keeping what was typed', () => {
        renderModal()
        fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Клуб' } })
        fireEvent.change(screen.getByLabelText('Ссылка на чат'), { target: { value: 'club' } })
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(screen.getByLabelText('Название')).toHaveValue('Клуб')
    })
})
