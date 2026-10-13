import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { useSession } from '@/entities/session'

import { GuestOnly, RequireAuth } from './guards'

function renderAt(route: string) {
    return render(
        <MemoryRouter initialEntries={[route]}>
            <Routes>
                <Route element={<GuestOnly />}>
                    <Route path="/auth/login" element={<p>вход</p>} />
                </Route>
                <Route element={<RequireAuth />}>
                    <Route path="/" element={<p>главная</p>} />
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

    it('shows a loader instead of any page while the session is unknown', () => {
        useSession.setState({ status: 'unknown' })
        renderAt('/')
        expect(screen.queryByText('главная')).toBeNull()
        expect(screen.queryByText('вход')).toBeNull()
        expect(screen.getByRole('status')).toBeInTheDocument()
    })
})
