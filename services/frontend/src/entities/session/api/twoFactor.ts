import { request } from '@/shared/api'

export type TwoFactorMethod = 'email' | 'totp'

export interface TwoFactorStatus {
    method: TwoFactorMethod
    totp_enabled: boolean
    backup_codes_remaining: number
}

export interface SetupTOTPResponse {
    secret: string
    otpauth_url: string
}

export interface BackupCodesResponse {
    backup_codes: string[]
}

export function getTwoFactor(): Promise<TwoFactorStatus> {
    return request<TwoFactorStatus>('/auth/2fa')
}

export function setupTOTP(password: string): Promise<SetupTOTPResponse> {
    return request<SetupTOTPResponse>('/auth/2fa/totp/setup', {
        method: 'POST',
        body: { password },
    })
}

export function confirmTOTP(code: string): Promise<BackupCodesResponse> {
    return request<BackupCodesResponse>('/auth/2fa/totp/confirm', {
        method: 'POST',
        body: { code },
    })
}

export function switchTwoFactorMethod(body: {
    method: TwoFactorMethod
    password: string
    code?: string
}): Promise<{ method: TwoFactorMethod }> {
    return request<{ method: TwoFactorMethod }>('/auth/2fa/method', {
        method: 'POST',
        body,
    })
}

export function regenerateBackupCodes(body: {
    password: string
    code: string
}): Promise<BackupCodesResponse> {
    return request<BackupCodesResponse>('/auth/2fa/backup-codes/regenerate', {
        method: 'POST',
        body,
    })
}

export function deleteAccount(body: { password: string; code?: string }): Promise<void> {
    return request<void>('/auth/account', {
        method: 'DELETE',
        body,
    })
}
