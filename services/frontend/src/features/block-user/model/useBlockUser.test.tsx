import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'

import { useBlockUser } from './useBlockUser'

describe('useBlockUser', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it.each(['block', 'unblock'] as const)('%s marks my black list as outdated', async (action) => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => Promise.resolve(new Response('{"message":"ok"}'))),
        )
        const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
        // чёрный список уже загружен: после действия он должен перечитаться
        client.setQueryData(['blocked'], { pages: [[]], pageParams: [0] })
        const wrapper = ({ children }: { children: ReactNode }) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        )
        const { result } = renderHook(() => useBlockUser('u1'), { wrapper })

        await act(() => result.current[action].mutateAsync())
        expect(client.getQueryState(['blocked'])?.isInvalidated).toBe(true)
    })
})
