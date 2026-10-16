import { useState } from 'react'

import { LoginForm } from '@/features/auth/login'
import { OtpForm } from '@/features/auth/otp'
import { notify } from '@/shared/lib'
import { AuthCard, AuthLayout } from '@/widgets/auth-layout'

interface Challenge {
    challengeId: string
    email: string
}

// почта и пароль, затем код из письма
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
