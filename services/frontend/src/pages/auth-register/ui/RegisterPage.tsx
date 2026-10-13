import { useState } from 'react'
import { useNavigate } from 'react-router'

import { RegisterForm } from '@/features/auth-register'
import { ROUTES } from '@/shared/config'
import { Button } from '@/shared/ui'
import { AuthCard, AuthLayout, AuthStatus } from '@/widgets/auth-layout'

export function RegisterPage() {
    const navigate = useNavigate()
    const [sentTo, setSentTo] = useState<string | null>(null)

    if (sentTo) {
        return (
            <AuthLayout>
                <AuthCard>
                    <AuthStatus
                        state="success"
                        actions={<Button onClick={() => navigate(ROUTES.login)}>Ко входу</Button>}
                    >
                        Мы отправили ссылку для подтверждения на {sentTo}. Перейдите по ней, чтобы
                        завершить регистрацию.
                    </AuthStatus>
                </AuthCard>
            </AuthLayout>
        )
    }

    // на шаге формы логотип внутри карточки, как на экране входа
    return (
        <AuthLayout showLogo={false}>
            <AuthCard onBack={() => navigate(ROUTES.login)}>
                <RegisterForm onSuccess={setSentTo} />
            </AuthCard>
        </AuthLayout>
    )
}
