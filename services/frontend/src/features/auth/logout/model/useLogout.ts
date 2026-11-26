import { useMutation, useQueryClient } from '@tanstack/react-query'

import { useDrafts } from '@/entities/message'
import { useSession } from '@/entities/session'

import { logout } from '../api/logout'

// выход локально выполняется в любом случае: даже если сервер недоступен, пользователь хочет выйти
export function useLogout() {
    const queryClient = useQueryClient()
    const signOut = useSession((state) => state.signOut)

    return useMutation({
        mutationFn: logout,
        onSettled: () => {
            signOut()
            useDrafts.getState().clearAll()
            // данные прошлого пользователя не должны остаться в кеше
            queryClient.clear()
        },
    })
}
