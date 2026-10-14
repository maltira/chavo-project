// пути приложения; register/verify и reset/confirm фиксированы ссылками из писем auth-service
export const ROUTES = {
    home: '/',
    login: '/auth/login',
    register: '/auth/register',
    registerVerify: '/auth/register/verify',
    reset: '/auth/reset',
    resetConfirm: '/auth/reset/confirm',
    profileSetup: '/profile/setup',
    chat: '/chats/:id',
} as const

export const chatPath = (id: string) => `/chats/${encodeURIComponent(id)}`
