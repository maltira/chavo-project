import { request } from '@/shared/api'

// письмо со ссылкой подтверждения уходит на почту; войти можно только после перехода по ней
export function register(body: { email: string; password: string }) {
    return request<{ message: string }>('/auth/register', { method: 'POST', body })
}
