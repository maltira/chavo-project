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
    // ссылка на чат: код открытой группы или сгенерированный код закрытой
    chatLink: '/chat/:code',
    // справочные страницы: открываются в новой вкладке, доступны без входа
    faq: '/faq',
    terms: '/terms',
    privacy: '/privacy',
} as const

export const chatPath = (id: string) => `/chats/${encodeURIComponent(id)}`
export const newChatPath = (userId: string) => `/chats/new/${encodeURIComponent(userId)}`
export const chatLinkPath = (code: string) => `/chat/${encodeURIComponent(code)}`

// ссылка на чат целиком (для копирования) и без протокола (для показа)
export const chatLinkUrl = (code: string) => `${window.location.origin}${chatLinkPath(code)}`
export const chatLinkText = (code: string) => `${window.location.host}${chatLinkPath(code)}`
