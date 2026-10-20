import { createBrowserRouter } from 'react-router'

import { LoginPage } from '@/pages/auth-login'
import { RegisterPage } from '@/pages/auth-register'
import { RegisterVerifyPage } from '@/pages/auth-register-verify'
import { ResetPage } from '@/pages/auth-reset'
import { ResetConfirmPage } from '@/pages/auth-reset-confirm'
import { ChatsPage } from '@/pages/chats'
import { HomePage } from '@/pages/home'
import { FaqPage, PrivacyPage, TermsPage } from '@/pages/info'
import { ProfileSetupPage } from '@/pages/profile-setup'
import { ROUTES } from '@/shared/config'

import { AppRoot } from './guards'
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
    // справочные страницы - в любом состоянии сессии
    { path: ROUTES.faq, element: <FaqPage /> },
    { path: ROUTES.terms, element: <TermsPage /> },
    { path: ROUTES.privacy, element: <PrivacyPage /> },
    {
        element: <RequireAuth />,
        children: [
            {
                element: <RequireNoProfile />,
                children: [{ path: ROUTES.profileSetup, element: <ProfileSetupPage /> }],
            },
            {
                element: <RequireProfile />,
                children: [
                    {
                        element: <AppRoot />,
                        children: [
                            { path: ROUTES.home, element: <ChatsPage /> },
                            { path: ROUTES.chat, element: <ChatsPage /> },
                            { path: ROUTES.newChat, element: <ChatsPage /> },
                            { path: ROUTES.chatLink, element: <ChatsPage /> },
                            // песочница компонентов: только в разработке
                            ...(import.meta.env.DEV
                                ? [{ path: '/ui', element: <HomePage /> }]
                                : []),
                        ],
                    },
                ],
            },
        ],
    },
])
