import { getAccessToken, request } from '@/shared/api'

// сеанс - вход на устройстве; id совпадает с sid в access token
export interface Session {
    id: string
    device_name?: string
    user_agent?: string
    ip_address?: string
    created_at: string
    expires_at: string
}

export function getSessions(): Promise<Session[]> {
    return request<Session[]>('/auth/sessions')
}

export function terminateSession(id: string): Promise<void> {
    return request<void>(`/auth/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// после смены пароля сервер завершает все сеансы, включая текущий
export function changePassword(currentPassword: string, newPassword: string): Promise<void> {
    return request<void>('/auth/change-password', {
        method: 'POST',
        body: { current_password: currentPassword, new_password: newPassword },
    })
}

// id текущего сеанса из access token (claim sid); подпись не проверяем - это лишь подсказка интерфейсу
export function currentSessionId(): string | null {
    const token = getAccessToken()
    const payload = token?.split('.')[1]
    if (!payload) {
        return null
    }
    try {
        const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
        const sid: unknown = (JSON.parse(json) as { sid?: unknown }).sid
        return typeof sid === 'string' ? sid : null
    } catch {
        return null
    }
}
