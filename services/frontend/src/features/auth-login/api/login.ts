import { request } from '@/shared/api'

export interface LoginResponse {
    challenge_id: string
    message: string
}

export function login(body: { email: string; password: string }) {
    return request<LoginResponse>('/auth/login', { method: 'POST', body })
}
