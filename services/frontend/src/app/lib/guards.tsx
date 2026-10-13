import { Navigate, Outlet } from 'react-router'

import { useSession } from '@/entities/session'
import { useMe } from '@/entities/user'
import { ROUTES } from '@/shared/config'

import { Splash } from './Splash'

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

// главная и всё остальное приложение: без профиля API отвечает PROFILE_REQUIRED
export function RequireProfile() {
    const me = useMe()
    if (me.isPending || me.isError) {
        return <Splash retrying={me.failureCount > 0} failed={me.isError} />
    }
    return me.data ? <Outlet /> : <Navigate to={ROUTES.profileSetup} replace />
}

// экран создания профиля: если профиль уже есть, делать там нечего
export function RequireNoProfile() {
    const me = useMe()
    if (me.isPending || me.isError) {
        return <Splash retrying={me.failureCount > 0} failed={me.isError} />
    }
    return me.data ? <Navigate to={ROUTES.home} replace /> : <Outlet />
}
