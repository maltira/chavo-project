import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'

import { useNotifications } from '@/shared/lib'

import { useTerminateSessions } from './useTerminateSessions'

const fetchMock = vi.fn()

function setup() {
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    return renderHook(() => useTerminateSessions(), { wrapper })
}

const titles = () => useNotifications.getState().items.map((n) => n.title)

describe('useTerminateSessions', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
        useNotifications.setState({ items: [] })
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('reports a partial success instead of a failure', async () => {
        fetchMock.mockImplementation((url: string) =>
            Promise.resolve(
                url.endsWith('/s2')
                    ? new Response('{"error":"Запись не найдена"}', { status: 404 })
                    : new Response('{"message":"ok"}'),
            ),
        )
        const { result } = setup()
        await act(() => result.current.mutateAsync(['s1', 's2', 's3']))
        expect(titles()).toContain('Завершено 2 из 3')
    })

    it('says all sessions ended when every request succeeded', async () => {
        // у каждого запроса свой ответ: тело Response читается один раз
        fetchMock.mockImplementation(() => Promise.resolve(new Response('{"message":"ok"}')))
        const { result } = setup()
        await act(() => result.current.mutateAsync(['s1', 's2']))
        expect(titles()).toContain('Сеансы завершены')
    })
})
