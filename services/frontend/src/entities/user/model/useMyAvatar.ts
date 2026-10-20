import { useMutation, useQueryClient } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import { deleteMyAvatar, uploadMyAvatar } from '../api/user'
import type { Profile } from './types'
import { userKey } from './useUser'

// загрузка и удаление своего фото: свой профиль обновляется сразу, у собеседников - при следующей загрузке
export function useMyAvatar() {
    const queryClient = useQueryClient()
    const put = (avatar_url: string | undefined) => {
        const me = queryClient.getQueryData<Profile | null>(['me'])
        queryClient.setQueryData<Profile | null>(['me'], (p) => (p ? { ...p, avatar_url } : p))
        if (me) {
            void queryClient.invalidateQueries({ queryKey: userKey(me.user_id) })
        }
    }
    const onError = (error: unknown) =>
        notify({
            icon: 'warn',
            title: 'Не удалось сохранить фото',
            description: errorMessage(error),
        })

    const upload = useMutation({
        mutationFn: uploadMyAvatar,
        onSuccess: ({ avatar_url }) => {
            put(avatar_url)
            notify({ icon: 'check', title: 'Фото обновлено' })
        },
        onError,
    })
    const remove = useMutation({
        mutationFn: deleteMyAvatar,
        onSuccess: () => {
            put(undefined)
            notify({ icon: 'check', title: 'Фото удалено' })
        },
        onError,
    })

    return { upload, remove, pending: upload.isPending || remove.isPending }
}
