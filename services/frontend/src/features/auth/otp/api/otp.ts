import { request } from '@/shared/api'

export interface TokenResponse {
    access_token: string
    token_type: string
}

export function verifyOtp(body: { challenge_id: string; code: string }) {
    return request<TokenResponse>('/auth/otp/verify', { method: 'POST', body })
}

export function resendOtp(body: { challenge_id: string }) {
    return request<{ message: string }>('/auth/otp/resend', { method: 'POST', body })
}
