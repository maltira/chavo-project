import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useParams } from 'react-router'

import { useGroupCard } from '@/entities/conversation'
import { useNotifications } from '@/shared/lib'

import { useChatLink } from './useChatLink'

function LinkPage() {
    useChatLink(useParams().code)
    return <p>ссылка</p>
}

function renderAt(route: string) {
    render(
        <QueryClientProvider client={new QueryClient()}>
            <MemoryRouter initialEntries={[route]}>
                <Routes>
                    <Route path="/" element={<p>главная</p>} />
                    <Route path="/chat/:code" element={<LinkPage />} />
                    <Route path="/chats/:id" element={<p>чат группы</p>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

const info = {
    conversation_id: 'g1',
    visibility: 'private',
    slug: null,
    name: 'Команда',
    description: null,
    avatar_url: null,
    members_count: 3,
    is_member: false,
    request_status: null,
    status: null,
    avatar_shape: null,
}

describe('useChatLink', () => {
    beforeEach(() => {
        useGroupCard.setState({ target: null })
        useNotifications.setState({ items: [] })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('opens the group window with the link code for a stranger', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => Promise.resolve(new Response(JSON.stringify(info)))),
        )
        renderAt('/chat/abc123')
        expect(await screen.findByText('главная')).toBeInTheDocument()
        expect(useGroupCard.getState().target).toMatchObject({
            kind: 'preview',
            group: { id: 'g1', visibility: 'private', code: 'abc123' },
        })
    })

    it('takes a member straight to the chat', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() =>
                Promise.resolve(new Response(JSON.stringify({ ...info, is_member: true }))),
            ),
        )
        renderAt('/chat/abc123')
        expect(await screen.findByText('чат группы')).toBeInTheDocument()
        expect(useGroupCard.getState().target).toBeNull()
    })

    it('tells when the link no longer works', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() =>
                Promise.resolve(
                    new Response(JSON.stringify({ error: 'Запись не найдена' }), { status: 404 }),
                ),
            ),
        )
        renderAt('/chat/old')
        expect(await screen.findByText('главная')).toBeInTheDocument()
        expect(useNotifications.getState().items[0]?.title).toBe('Ссылка недействительна')
    })
})
