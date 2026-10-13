import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { MemoryRouter } from 'react-router'

import { RegisterVerifyPage } from './RegisterVerifyPage'

function renderAt(url: string) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <StrictMode>
            <QueryClientProvider client={client}>
                <MemoryRouter initialEntries={[url]}>
                    <RegisterVerifyPage />
                </MemoryRouter>
            </QueryClientProvider>
        </StrictMode>,
    )
}

describe('RegisterVerifyPage', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('confirms the email with a single request even in StrictMode', async () => {
        const fetchMock = vi.fn().mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        renderAt('/auth/register/verify?token=abc')

        expect(screen.getByText('Выполняем ваш запрос...')).toBeInTheDocument()
        expect(await screen.findByText(/Почта подтверждена/)).toBeInTheDocument()
        expect(fetchMock).toHaveBeenCalledTimes(1)
        expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/register/verify?token=abc')
    })

    it('explains an invalid link', async () => {
        vi.stubGlobal(
            'fetch',
            vi
                .fn()
                .mockResolvedValue(
                    new Response('{"error":"x","reason":"INVALID_TOKEN"}', { status: 400 }),
                ),
        )
        renderAt('/auth/register/verify?token=old')
        expect(await screen.findByRole('alert')).toHaveTextContent('Ссылка недействительна')
    })

    it('does not call the server without a token', () => {
        const fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)
        renderAt('/auth/register/verify')
        expect(screen.getByRole('alert')).toHaveTextContent('Ссылка недействительна')
        expect(fetchMock).not.toHaveBeenCalled()
    })
})
