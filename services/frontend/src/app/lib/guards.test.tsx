import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { useSession } from '@/entities/session'

import { getMe } from '@/entities/user'

import { GuestOnly, RequireAuth, RequireNoProfile, RequireProfile } from './guards'

// useMe берёт getMe прямо из api/me, поэтому подменяется именно этот модуль
vi.mock('@/entities/user/api/me', () => ({ getMe: vi.fn() }))

function renderAt(route: string) {
    return render(
        <MemoryRouter initialEntries={[route]}>
            <Routes>
                <Route element={<GuestOnly />}>
                    <Route path="/auth/login" element={<p>вход</p>} />
                </Route>
                <Route element={<RequireAuth />}>
                    <Route path="/" element={<p>главная</p>} />
                    <Route path="/chat/:code" element={<p>ссылка на чат</p>} />
                </Route>
            </Routes>
        </MemoryRouter>,
    )
}

describe('route guards', () => {
    it('sends a guest from a private page to login', () => {
        useSession.setState({ status: 'guest' })
        renderAt('/')
        expect(screen.getByText('вход')).toBeInTheDocument()
    })

    it('sends a signed-in user from login to the home page', () => {
        useSession.setState({ status: 'authenticated' })
        renderAt('/auth/login')
        expect(screen.getByText('главная')).toBeInTheDocument()
    })

    it('returns a guest to the chat link after signing in', () => {
        useSession.setState({ status: 'guest' })
        renderAt('/chat/book_club')
        expect(screen.getByText('вход')).toBeInTheDocument()
        act(() => useSession.setState({ status: 'authenticated' }))
        expect(screen.getByText('ссылка на чат')).toBeInTheDocument()
    })

    it('shows a loader instead of any page while the session is unknown', () => {
        useSession.setState({ status: 'unknown' })
        renderAt('/')
        expect(screen.queryByText('главная')).toBeNull()
        expect(screen.queryByText('вход')).toBeNull()
        expect(screen.getByRole('status')).toBeInTheDocument()
    })
})

function renderProfileRoutes(route: string) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={[route]}>
                <Routes>
                    <Route element={<RequireNoProfile />}>
                        <Route path="/profile/setup" element={<p>создание профиля</p>} />
                    </Route>
                    <Route element={<RequireProfile />}>
                        <Route path="/" element={<p>главная</p>} />
                    </Route>
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('profile guards', () => {
    it('sends a user without a profile to profile setup', async () => {
        vi.mocked(getMe).mockResolvedValue(null)
        renderProfileRoutes('/')
        expect(await screen.findByText('создание профиля')).toBeInTheDocument()
    })

    it('sends a user with a profile from profile setup to the home page', async () => {
        vi.mocked(getMe).mockResolvedValue({ username: 'me' } as never)
        renderProfileRoutes('/profile/setup')
        expect(await screen.findByText('главная')).toBeInTheDocument()
    })
})
