import { ApiError } from './http'

// тексты для кодов, общих для всех форм; специфичные коды форма передаёт сама
const COMMON: Record<string, string> = {
    RATE_LIMITED: 'Слишком много попыток, подождите минуту',
    SERVICE_UNAVAILABLE: 'Сервис временно недоступен, попробуйте позже',
    NETWORK_ERROR: 'Не удалось выполнить запрос, проверьте соединение',
}

export function errorMessage(error: unknown, byReason: Record<string, string> = {}) {
    if (error instanceof ApiError) {
        const reason = error.reason
        if (reason && byReason[reason]) {
            return byReason[reason]
        }
        if (reason && COMMON[reason]) {
            return COMMON[reason]
        }
        return error.message
    }
    return 'Не удалось выполнить запрос, проверьте соединение'
}
