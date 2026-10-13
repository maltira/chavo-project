import { createBrowserRouter } from 'react-router'

import { LoginPage } from '@/pages/auth-login'
import { HomePage } from '@/pages/home'
import { ROUTES } from '@/shared/config'

import { GuestOnly, RequireAuth } from './guards'

export const router = createBrowserRouter([
    {
        element: <GuestOnly />,
        children: [{ path: ROUTES.login, element: <LoginPage /> }],
    },
    {
        element: <RequireAuth />,
        children: [{ path: ROUTES.home, element: <HomePage /> }],
    },
])
