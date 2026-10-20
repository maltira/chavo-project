import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'

import { StatusEditor } from './StatusEditor'

const fetchMock = vi.fn()

function renderEditor(status: object | null) {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { user_id: 'me', status })
    client.setQueryData(['customization'], {
        status: { icons: ['moon', 'flame'], durations: ['1h', '24h', 'never'] },
        avatar_shapes: [],
    })
    const onClose = vi.fn()
    render(
        <QueryClientProvider client={client}>
            <StatusEditor onDone={onClose} />
        </QueryClientProvider>,
    )
    return { client, onClose }
}

describe('StatusEditor', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('sets a whole new status and puts it into my profile', async () => {
        const saved = {
            icon: 'flame',
            text: 'Дедлайн',
            background: '#25E87F',
            foreground: '#FAF9F5',
            expires_at: null,
        }
        fetchMock.mockResolvedValue(new Response(JSON.stringify(saved), { status: 201 }))
        const { client, onClose } = renderEditor(null)

        expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
        expect(screen.queryByRole('button', { name: 'Снять статус' })).toBeNull()

        fireEvent.click(screen.getByRole('radio', { name: 'flame' }))
        fireEvent.change(screen.getByPlaceholderText(/Сегодня отдыхаю/), {
            target: { value: '  Дедлайн  ' },
        })
        const background = screen.getByRole('radiogroup', { name: 'Фон' })
        const foreground = screen.getByRole('radiogroup', { name: 'Цвет текста и иконки' })
        fireEvent.click(within(background).getByRole('radio', { name: '#25E87F' }))
        fireEvent.click(within(foreground).getByRole('radio', { name: '#FAF9F5' }))
        fireEvent.click(screen.getByRole('radio', { name: 'Всегда' }))
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

        await vi.waitFor(() => expect(onClose).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me/status')
        expect(JSON.parse(init.body as string)).toEqual({
            icon: 'flame',
            text: 'Дедлайн',
            background: '#25E87F',
            foreground: '#FAF9F5',
            duration: 'never',
        })
        expect(client.getQueryData<{ status: unknown }>(['me'])?.status).toEqual(saved)
    })

    it('clears the current status', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        const { client } = renderEditor({
            icon: 'moon',
            text: 'Сплю',
            background: '#2A2A2A',
            foreground: '#FAF9F5',
            expires_at: null,
        })
        expect(screen.getByDisplayValue('Сплю')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Снять статус' }))
        await vi.waitFor(() =>
            expect(client.getQueryData<{ status: unknown }>(['me'])?.status).toBeNull(),
        )
        expect(fetchMock.mock.calls[0]![1].method).toBe('DELETE')
    })
})

describe('StatusEditor for a group', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('starts from the group status and saves it into the chat card', async () => {
        const current = {
            icon: 'moon',
            text: 'Тихий час',
            background: '#2A2A2A',
            foreground: '#FAF9F5',
            expires_at: null,
        }
        const client = new QueryClient({
            defaultOptions: { queries: { retry: false, staleTime: Infinity } },
        })
        client.setQueryData(['me'], { user_id: 'me', status: null })
        client.setQueryData(['conversation', 'g1'], { id: 'g1', status: current })
        client.setQueryData(['customization'], {
            status: { icons: ['moon', 'flame'], durations: ['24h'] },
            avatar_shapes: [],
        })
        const saved = { ...current, text: 'Наводим тишину' }
        fetchMock.mockImplementation(() =>
            Promise.resolve(new Response(JSON.stringify(saved), { status: 201 })),
        )
        const onDone = vi.fn()
        render(
            <QueryClientProvider client={client}>
                <StatusEditor target={{ kind: 'group', id: 'g1' }} onDone={onDone} />
            </QueryClientProvider>,
        )

        const text = screen.getByPlaceholderText(/Сегодня отдыхаю/)
        expect(text).toHaveValue('Тихий час')
        fireEvent.change(text, { target: { value: 'Наводим тишину' } })
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

        await vi.waitFor(() => expect(onDone).toHaveBeenCalled())
        expect(fetchMock.mock.calls[0]![0]).toBe('/api/conversations/g1/status')
        expect(client.getQueryData<{ status: unknown }>(['conversation', 'g1'])?.status).toEqual(
            saved,
        )
        // мой статус не тронут
        expect(client.getQueryData<{ status: unknown }>(['me'])?.status).toBeNull()
    })
})
