import { useMutation, useQueryClient } from '@tanstack/react-query'

import { blockStatusKey, blockUser, unblockUser, userKey } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

// блокировка и разблокировка; после них перечитываются статус блокировки и профиль
// (заблокированному сервер не отдаёт аватар, статус и онлайн)
export function useBlockUser(userId: string) {
    const queryClient = useQueryClient()

    const refresh = () => {
        void queryClient.invalidateQueries({ queryKey: blockStatusKey(userId) })
        void queryClient.invalidateQueries({ queryKey: userKey(userId) })
    }
    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })

    const block = useMutation({
        mutationFn: () => blockUser(userId),
        onSuccess: () => {
            refresh()
            notify({ icon: 'block', title: 'Пользователь заблокирован' })
        },
        onError,
    })
    const unblock = useMutation({
        mutationFn: () => unblockUser(userId),
        onSuccess: () => {
            refresh()
            notify({ icon: 'check', title: 'Пользователь разблокирован' })
        },
        onError,
    })

    return { block, unblock, pending: block.isPending || unblock.isPending }
}
