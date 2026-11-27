import type { TwoFactorMethod } from '@/entities/session'
import { request } from '@/shared/api'

export interface LoginResponse {
    challenge_id: string
    method?: TwoFactorMethod
    message: string
}

export function login(body: { email: string; password: string }) {
    return request<LoginResponse>('/auth/login', { method: 'POST', body })
}
