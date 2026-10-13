import { useNavigate, useSearchParams } from 'react-router'

import { useVerifyEmail } from '@/features/auth-verify-email'
import { ROUTES } from '@/shared/config'
import { Button } from '@/shared/ui'
import { AuthCard, AuthLayout, AuthStatus } from '@/widgets/auth-layout'

// страница по ссылке из письма: /auth/register/verify?token=...
export function RegisterVerifyPage() {
    const navigate = useNavigate()
    const [params] = useSearchParams()
    const token = params.get('token')
    const verify = useVerifyEmail(token)

    const toLogin = <Button onClick={() => navigate(ROUTES.login)}>Войти</Button>

    let content
    if (!token || verify.isError) {
        content = (
            <AuthStatus state="error" actions={toLogin}>
                Ссылка недействительна или устарела. Если почта уже подтверждена, просто войдите.
            </AuthStatus>
        )
    } else if (verify.isSuccess) {
        content = (
            <AuthStatus state="success" actions={toLogin}>
                Почта подтверждена. Теперь можно войти в аккаунт.
            </AuthStatus>
        )
    } else {
        content = <AuthStatus state="loading">Выполняем ваш запрос...</AuthStatus>
    }

    return (
        <AuthLayout>
            <AuthCard>{content}</AuthCard>
        </AuthLayout>
    )
}
