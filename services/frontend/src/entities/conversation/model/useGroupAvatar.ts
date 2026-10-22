import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import {
    deleteGroupAvatar,
    deleteGroupAvatarPhoto,
    getGroupAvatarPhotos,
    selectGroupAvatarPhoto,
    uploadGroupAvatar,
} from '../api/groups'
import { invalidateGroup } from './useGroupLists'

export const groupAvatarPhotosKey = (groupId: string) => ['group-avatar-photos', groupId] as const

// история аватарок группы; грузится, когда фото открыли на весь экран (enabled)
export function useGroupAvatarPhotos(groupId: string, enabled: boolean) {
    return useQuery({
        queryKey: groupAvatarPhotosKey(groupId),
        queryFn: () => getGroupAvatarPhotos(groupId),
        enabled,
    })
}

// загрузка, смена главного и удаление фото группы (только админ); у участников обновится по событию
// conversation.updated; история перечитывается после каждого изменения
export function useGroupAvatar(groupId: string) {
    const queryClient = useQueryClient()
    const changed = () => {
        invalidateGroup(queryClient, groupId)
        void queryClient.invalidateQueries({ queryKey: groupAvatarPhotosKey(groupId) })
    }
    const onError = (error: unknown) =>
        notify({
            icon: 'warn',
            title: 'Не удалось сохранить фото',
            description: errorMessage(error),
        })

    const upload = useMutation({
        mutationFn: (photo: Blob) => uploadGroupAvatar(groupId, photo),
        onSuccess: () => {
            changed()
            notify({ icon: 'check', title: 'Фото группы обновлено' })
        },
        onError,
    })
    // убирает текущее фото: текущим становится предыдущее, если оно есть
    const remove = useMutation({
        mutationFn: () => deleteGroupAvatar(groupId),
        onSuccess: () => {
            changed()
            notify({ icon: 'check', title: 'Фото группы удалено' })
        },
        onError,
    })
    // именно это фото из истории
    const removePhoto = useMutation({
        mutationFn: (photoId: string) => deleteGroupAvatarPhoto(groupId, photoId),
        onSuccess: () => {
            changed()
            notify({ icon: 'check', title: 'Фото удалено' })
        },
        onError,
    })
    const select = useMutation({
        mutationFn: (photoId: string) => selectGroupAvatarPhoto(groupId, photoId),
        onSuccess: () => {
            changed()
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
