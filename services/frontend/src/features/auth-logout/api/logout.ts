import { request } from '@/shared/api'

// отзывает сессию по refresh-cookie; сервер чистит cookie
export function logout() {
    return request<{ message: string }>('/auth/logout', { method: 'POST' })
}
