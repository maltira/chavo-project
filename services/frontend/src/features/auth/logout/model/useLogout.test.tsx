import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'

import { useSession } from '@/entities/session'
import { ApiError } from '@/shared/api'

import { logout } from '../api/logout'
import { useLogout } from './useLogout'

vi.mock('../api/logout', () => ({ logout: vi.fn() }))

const logoutMock = vi.mocked(logout)

function setup() {
    const client = new QueryClient()
    client.setQueryData(['me'], { username: 'me' })
    const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    return { client, ...renderHook(() => useLogout(), { wrapper }) }
}

describe('useLogout', () => {
    beforeEach(() => {
        logoutMock.mockReset()
        useSession.getState().signIn('tok')
    })

    it('signs out and clears cached data', async () => {
        logoutMock.mockResolvedValue({ message: 'ok' })
        const { client, result } = setup()
        act(() => result.current.mutate())

        await waitFor(() => expect(useSession.getState().status).toBe('guest'))
        expect(client.getQueryData(['me'])).toBeUndefined()
    })

    it('signs out locally even if the server is unreachable', async () => {
        logoutMock.mockRejectedValue(new ApiError(0, 'x', 'NETWORK_ERROR'))
        const { result } = setup()
        act(() => result.current.mutate())

        await waitFor(() => expect(useSession.getState().status).toBe('guest'))
    })
})
