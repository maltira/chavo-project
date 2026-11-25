import { useMutation, useQueryClient } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import { deleteAvatarPhoto, deleteMyAvatar, selectAvatarPhoto, uploadMyAvatar } from '../api/user'
import type { Profile } from './types'
import { avatarPhotosKey } from './useAvatarPhotos'
import { userKey } from './useUser'

// загрузка, смена главного и удаление своих фото: свой профиль обновляется сразу, у собеседников - при
// следующей загрузке; история перечитывается после каждого изменения
export function useMyAvatar() {
    const queryClient = useQueryClient()
    // avatar_url - новое текущее фото, если известно; иначе профиль перечитывается
    const put = (avatar_url?: string) => {
        const me = queryClient.getQueryData<Profile | null>(['me'])
        if (avatar_url !== undefined) {
            queryClient.setQueryData<Profile | null>(['me'], (p) => (p ? { ...p, avatar_url } : p))
        } else {
            void queryClient.invalidateQueries({ queryKey: ['me'] })
        }
        if (me) {
            void queryClient.invalidateQueries({ queryKey: userKey(me.user_id) })
            void queryClient.invalidateQueries({ queryKey: avatarPhotosKey(me.user_id) })
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
    // убирает текущее фото: текущим становится предыдущее, если оно есть
    const remove = useMutation({
        mutationFn: deleteMyAvatar,
        onSuccess: () => {
            put()
            notify({ icon: 'check', title: 'Фото удалено' })
        },
        onError,
    })
    // именно это фото из истории
    const removePhoto = useMutation({
        mutationFn: deleteAvatarPhoto,
        onSuccess: () => {
            put()
            notify({ icon: 'check', title: 'Фото удалено' })
        },
        onError,
    })
    const select = useMutation({
        mutationFn: selectAvatarPhoto,
        onSuccess: () => {
            put()
            notify({ icon: 'check', title: 'Фото сделано главным' })
        },
        onError,
    })

    return {
        upload,
        remove,
        removePhoto,
        select,
        pending: upload.isPending || remove.isPending || removePhoto.isPending || select.isPending,
    }
}
