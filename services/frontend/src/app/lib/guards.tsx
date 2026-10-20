import { Navigate, Outlet, useLocation } from 'react-router'

import { useSession } from '@/entities/session'
import { useMe } from '@/entities/user'
import { ROUTES } from '@/shared/config'

import { Splash } from './Splash'
import { RealtimeBridge } from './RealtimeBridge'

// куда вернуть после входа и создания профиля (например, ссылка на чат, открытая гостем)
interface ReturnState {
    from?: string
}

function useReturnTo() {
    const location = useLocation()
    return (location.state as ReturnState | null)?.from ?? ROUTES.home
}

function useHere(): ReturnState {
    const location = useLocation()
    return { from: location.pathname + location.search }
}

// закрытые страницы: гость уходит на вход и после него возвращается сюда
export function RequireAuth() {
    const status = useSession((state) => state.status)
    const unreachable = useSession((state) => state.unreachable)
    const here = useHere()
    if (status === 'unknown') {
        return <Splash unreachable={unreachable} />
    }
    return status === 'authenticated' ? (
        <Outlet />
    ) : (
        <Navigate to={ROUTES.login} replace state={here} />
    )
}

// страницы входа: вошедший пользователь уходит на главную
export function GuestOnly() {
    const status = useSession((state) => state.status)
    const unreachable = useSession((state) => state.unreachable)
    const returnTo = useReturnTo()
    if (status === 'unknown') {
        return <Splash unreachable={unreachable} />
    }
    return status === 'guest' ? <Outlet /> : <Navigate to={returnTo} replace />
}

// главная и всё остальное приложение: без профиля API отвечает PROFILE_REQUIRED
export function RequireProfile() {
    const me = useMe()
    const here = useHere()
    if (me.isPending || me.isError) {
        return <Splash retrying={me.failureCount > 0} failed={me.isError} />
    }
    return me.data ? <Outlet /> : <Navigate to={ROUTES.profileSetup} replace state={here} />
}

// экран создания профиля: если профиль уже есть, делать там нечего
export function RequireNoProfile() {
    const me = useMe()
    const returnTo = useReturnTo()
    if (me.isPending || me.isError) {
        return <Splash retrying={me.failureCount > 0} failed={me.isError} />
    }
    return me.data ? <Navigate to={returnTo} replace /> : <Outlet />
}

// корень закрытой части приложения: гарды уже пропустили, можно держать соединение
export function AppRoot() {
    return (
        <>
            <RealtimeBridge />
            <Outlet />
        </>
    )
}
