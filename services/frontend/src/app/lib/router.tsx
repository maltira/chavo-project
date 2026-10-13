import { createBrowserRouter } from 'react-router'

import { LoginPage } from '@/pages/auth-login'
import { RegisterPage } from '@/pages/auth-register'
import { RegisterVerifyPage } from '@/pages/auth-register-verify'
import { ResetPage } from '@/pages/auth-reset'
import { ResetConfirmPage } from '@/pages/auth-reset-confirm'
import { HomePage } from '@/pages/home'
import { ProfileSetupPage } from '@/pages/profile-setup'
import { ROUTES } from '@/shared/config'

import { GuestOnly, RequireAuth, RequireNoProfile, RequireProfile } from './guards'

export const router = createBrowserRouter([
    // вход, регистрация и запрос сброса - только для гостя
    {
        element: <GuestOnly />,
        children: [
            { path: ROUTES.login, element: <LoginPage /> },
            { path: ROUTES.register, element: <RegisterPage /> },
            { path: ROUTES.reset, element: <ResetPage /> },
        ],
    },
    // ссылки из писем открываются в любом состоянии сессии
    { path: ROUTES.registerVerify, element: <RegisterVerifyPage /> },
    { path: ROUTES.resetConfirm, element: <ResetConfirmPage /> },
    {
        element: <RequireAuth />,
        children: [
            {
                element: <RequireNoProfile />,
                children: [{ path: ROUTES.profileSetup, element: <ProfileSetupPage /> }],
            },
            {
                element: <RequireProfile />,
                children: [{ path: ROUTES.home, element: <HomePage /> }],
            },
        ],
    },
])
