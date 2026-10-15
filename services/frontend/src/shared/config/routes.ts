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
    // личный чат, которого ещё нет: создаётся первым сообщением
    newChat: '/chats/new/:userId',
} as const

export const chatPath = (id: string) => `/chats/${encodeURIComponent(id)}`
export const newChatPath = (userId: string) => `/chats/new/${encodeURIComponent(userId)}`
