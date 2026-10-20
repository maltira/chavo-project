import { useMutation, useQueryClient } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import { deleteGroupAvatar, uploadGroupAvatar } from '../api/groups'
import { invalidateGroup } from './useGroupLists'

// загрузка и удаление фото группы (только админ); у участников обновится по событию conversation.updated
export function useGroupAvatar(groupId: string) {
    const queryClient = useQueryClient()
    const onError = (error: unknown) =>
        notify({
            icon: 'warn',
            title: 'Не удалось сохранить фото',
            description: errorMessage(error),
        })

    const upload = useMutation({
        mutationFn: (photo: Blob) => uploadGroupAvatar(groupId, photo),
        onSuccess: () => {
            invalidateGroup(queryClient, groupId)
            notify({ icon: 'check', title: 'Фото группы обновлено' })
        },
        onError,
    })
    const remove = useMutation({
        mutationFn: () => deleteGroupAvatar(groupId),
        onSuccess: () => {
            invalidateGroup(queryClient, groupId)
            notify({ icon: 'check', title: 'Фото группы удалено' })
        },
        onError,
    })

    return { upload, remove, pending: upload.isPending || remove.isPending }
}
