import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { useSession } from '@/entities/session'
import { ResetConfirmForm } from '@/features/auth/reset'
import { ROUTES } from '@/shared/config'
import { Button } from '@/shared/ui'
import { AuthCard, AuthLayout, AuthStatus } from '@/widgets/auth-layout'

type Result = 'form' | 'done' | 'invalid'

// страница по ссылке из письма: /auth/reset/confirm?token=...
export function ResetConfirmPage() {
    const navigate = useNavigate()
    const [params] = useSearchParams()
    const token = params.get('token')
    const signOut = useSession((state) => state.signOut)
    const [result, setResult] = useState<Result>(token ? 'form' : 'invalid')

    if (result === 'done') {
        return (
            <AuthLayout>
                <AuthCard>
                    <AuthStatus
                        state="success"
                        actions={<Button onClick={() => navigate(ROUTES.login)}>Войти</Button>}
                    >
                        Все активные сессии были завершены, повторите вход в аккаунт.
                    </AuthStatus>
                </AuthCard>
            </AuthLayout>
        )
    }

    if (result === 'invalid' || !token) {
        return (
            <AuthLayout>
                <AuthCard>
                    <AuthStatus
                        state="error"
                        actions={
                            <Button onClick={() => navigate(ROUTES.reset)}>
                                Запросить новую ссылку
                            </Button>
                        }
                    >
                        Ссылка недействительна или устарела.
                    </AuthStatus>
                </AuthCard>
            </AuthLayout>
        )
    }

    return (
        <AuthLayout>
            <AuthCard onBack={() => navigate(ROUTES.login)}>
                <ResetConfirmForm
                    token={token}
                    onInvalidToken={() => setResult('invalid')}
                    onSuccess={() => {
                        // сервер завершил все сессии, в том числе эту вкладку, если в ней был вход
                        signOut()
                        setResult('done')
                    }}
                />
            </AuthCard>
        </AuthLayout>
    )
}
