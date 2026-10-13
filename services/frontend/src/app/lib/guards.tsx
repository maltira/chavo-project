import { Navigate, Outlet } from 'react-router'

import { useSession } from '@/entities/session'
import { ROUTES } from '@/shared/config'
import { Loader } from '@/shared/ui'

// пока проверяется refresh-cookie, ни вход, ни закрытые страницы показывать нельзя
function Splash() {
    return (
        <div style={{ display: 'grid', placeItems: 'center', minHeight: '100dvh' }}>
            <Loader />
        </div>
    )
}

// закрытые страницы: гость уходит на вход
export function RequireAuth() {
    const status = useSession((state) => state.status)
    if (status === 'unknown') {
        return <Splash />
    }
    return status === 'authenticated' ? <Outlet /> : <Navigate to={ROUTES.login} replace />
}

// страницы входа: вошедший пользователь уходит на главную
export function GuestOnly() {
    const status = useSession((state) => state.status)
    if (status === 'unknown') {
        return <Splash />
    }
    return status === 'guest' ? <Outlet /> : <Navigate to={ROUTES.home} replace />
}
