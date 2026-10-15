import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { useGroupCard } from '@/entities/conversation'
import type { PublicGroup } from '@/entities/conversation'

import { GroupPreviewModal } from './GroupPreviewModal'

const group: PublicGroup = {
    id: 'g1',
    name: 'Книжный клуб',
    description: 'Читаем по книге в месяц',
    avatar_url: null,
    members_count: 4,
    is_member: false,
    status: null,
    avatar_shape: null,
}

const fetchMock = vi.fn()

function renderModal() {
    render(
        <QueryClientProvider client={new QueryClient()}>
            <MemoryRouter initialEntries={['/']}>
                <Routes>
                    <Route path="/" element={<GroupPreviewModal />} />
                    <Route path="/chats/:id" element={<p>чат группы</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('GroupPreviewModal', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        useGroupCard.setState({ group: null })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('joins the group and opens its chat', async () => {
        renderModal()
        act(() => useGroupCard.getState().open(group))
        expect(screen.getByRole('heading', { name: 'Книжный клуб' })).toBeInTheDocument()
        expect(screen.getByText('4 участника')).toBeInTheDocument()
        expect(screen.getByText('Читаем по книге в месяц')).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Вступить' }))
        expect(await screen.findByText('чат группы')).toBeInTheDocument()
        expect(fetchMock).toHaveBeenCalledWith(
            '/api/conversations/g1/join',
            expect.objectContaining({ method: 'POST' }),
        )
        expect(useGroupCard.getState().group).toBeNull()
    })

    it('opens the chat right away for a member', () => {
        renderModal()
        act(() => useGroupCard.getState().open({ ...group, is_member: true }))
        fireEvent.click(screen.getByRole('button', { name: 'Открыть чат' }))
        expect(screen.getByText('чат группы')).toBeInTheDocument()
        expect(fetchMock).not.toHaveBeenCalled()
    })
})
