import { useState } from 'react'
import { useNavigate } from 'react-router'

import { ResetRequestForm } from '@/features/auth-reset'
import { ROUTES } from '@/shared/config'
import { Button } from '@/shared/ui'
import { AuthCard, AuthLayout, AuthStatus } from '@/widgets/auth-layout'

export function ResetPage() {
    const navigate = useNavigate()
    const [message, setMessage] = useState<string | null>(null)

    return (
        <AuthLayout>
            {message ? (
                <AuthCard>
                    <AuthStatus
                        state="success"
                        actions={<Button onClick={() => navigate(ROUTES.login)}>Ко входу</Button>}
                    >
                        {message}
                    </AuthStatus>
                </AuthCard>
            ) : (
                <AuthCard onBack={() => navigate(ROUTES.login)}>
                    <ResetRequestForm onSuccess={setMessage} />
                </AuthCard>
            )}
        </AuthLayout>
    )
}
