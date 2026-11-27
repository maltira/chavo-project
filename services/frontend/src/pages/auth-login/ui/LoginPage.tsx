import { useState } from 'react'

import type { TwoFactorMethod } from '@/entities/session'
import { LoginForm } from '@/features/auth/login'
import { OtpForm } from '@/features/auth/otp'
import { notify } from '@/shared/lib'
import { AuthCard, AuthLayout } from '@/widgets/auth-layout'

interface Challenge {
    challengeId: string
    email: string
    method: TwoFactorMethod
}

// почта и пароль, затем код из письма или приложения-аутентификатора
export function LoginPage() {
    const [challenge, setChallenge] = useState<Challenge | null>(null)

    if (!challenge) {
        return (
            <AuthLayout showLogo={false}>
                <AuthCard>
                    <LoginForm
                        onSuccess={(next) => {
                            setChallenge(next)
                        }}
                    />
                </AuthCard>
            </AuthLayout>
        )
    }

    return (
        <AuthLayout>
            <AuthCard onBack={() => setChallenge(null)}>
                <OtpForm
                    challengeId={challenge.challengeId}
                    email={challenge.email}
                    method={challenge.method}
                    onExpired={() => {
                        notify({
                            id: 'otp-error',
                            icon: 'warn',
                            title: 'Ошибка входа',
                            description: 'Неверный или недействительный код, войдите заново',
                        })
                        setChallenge(null)
                    }}
                />
            </AuthCard>
        </AuthLayout>
    )
}
