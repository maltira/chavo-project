import { useMutation, useQueryClient } from '@tanstack/react-query'

import { clearMyStatus, setMyStatus } from '@/entities/user'
import type { Profile, Status, StatusInput } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

// установка и снятие своего статуса; профиль в кеше обновляется сразу, событий об этом нет
export function useMyStatus(onDone: () => void) {
    const queryClient = useQueryClient()
    const putStatus = (status: Status | null) =>
        queryClient.setQueryData<Profile | null>(['me'], (me) => (me ? { ...me, status } : me))
    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })

    const set = useMutation({
        mutationFn: (input: StatusInput) => setMyStatus(input),
        onSuccess: (status) => {
            putStatus(status)
            onDone()
        },
        onError,
    })
    const clear = useMutation({
        mutationFn: clearMyStatus,
        onSuccess: () => {
            putStatus(null)
            onDone()
        },
        onError,
    })

    return { set, clear, pending: set.isPending || clear.isPending }
}
