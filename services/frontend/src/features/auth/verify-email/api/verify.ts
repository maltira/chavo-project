import { request } from '@/shared/api'

// токен одноразовый: повторный вызов вернёт INVALID_TOKEN
export function verifyEmail(token: string) {
    return request<{ message: string }>(`/auth/register/verify?token=${encodeURIComponent(token)}`)
}
