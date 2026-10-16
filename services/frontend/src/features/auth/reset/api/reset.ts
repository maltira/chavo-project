import { request } from '@/shared/api'

// ответ одинаковый, есть аккаунт или нет: сервер не раскрывает, зарегистрирована ли почта
export function requestReset(body: { email: string }) {
    return request<{ message: string }>('/auth/reset/request', { method: 'POST', body })
}

// после успеха сервер завершает все сессии пользователя
export function confirmReset(body: { token: string; password: string }) {
    return request<{ message: string }>('/auth/reset/confirm', { method: 'POST', body })
}
