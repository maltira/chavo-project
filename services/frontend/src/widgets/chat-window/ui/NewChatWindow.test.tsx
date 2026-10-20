import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { NewChatWindow } from './NewChatWindow'

const fetchMock = vi.fn()

function renderWindow(blockStatus = { blocked_by_me: false, blocked_by_them: false }) {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { user_id: 'me' })
    client.setQueryData(['user', 'peer'], { user_id: 'peer', display_name: 'Герман Самойлов' })
    client.setQueryData(['block-status', 'peer'], blockStatus)
    render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={['/chats/new/peer']}>
                <Routes>
                    <Route path="/chats/new/:userId" element={<NewChatWindow userId="peer" />} />
                    <Route path="/chats/:id" element={<p>чат создан</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('NewChatWindow', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('sends the first message to the person and opens the created chat', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ id: 'm1', conversation_id: 'c-new' }), { status: 201 }),
        )
        renderWindow()
        expect(screen.getByRole('heading', { name: 'Герман Самойлов' })).toBeInTheDocument()

        const field = screen.getByLabelText('Сообщение')
        fireEvent.change(field, { target: { value: 'привет' } })
        fireEvent.keyDown(field, { key: 'Enter' })

        expect(await screen.findByText('чат создан')).toBeInTheDocument()
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/messages')
        expect(JSON.parse(init.body as string)).toEqual({
            recipient_id: 'peer',
            content: 'привет',
            reply_to_message_id: null,
        })
    })

    it('shows the notice instead of the field when the person blocked me', () => {
        renderWindow({ blocked_by_me: false, blocked_by_them: true })
        expect(screen.getByText('Пользователь ограничил отправку сообщений')).toBeInTheDocument()
        expect(screen.queryByLabelText('Сообщение')).toBeNull()
    })

    it('offers to unblock when I blocked the person', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        renderWindow({ blocked_by_me: true, blocked_by_them: false })
        expect(screen.getByText('Вы заблокировали пользователя')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Разблокировать' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/peer/block',
                expect.objectContaining({ method: 'DELETE' }),
            ),
        )
    })
})
