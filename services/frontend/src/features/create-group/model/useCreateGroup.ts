import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'

import { conversationsKey, createGroup, uploadGroupAvatar } from '@/entities/conversation'
import type { CreateGroupInput } from '@/entities/conversation'
import { ApiError, errorMessage } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

export interface CreateGroupRequest {
    input: CreateGroupInput
    // фото, выбранное в форме: загружается сразу после создания группы
    photo: Blob | null
}

// создание группы: после него группа сразу открывается; при ошибке форма остаётся как была
export function useCreateGroup(onCreated: () => void) {
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    return useMutation({
        mutationFn: async ({ input, photo }: CreateGroupRequest) => {
            const group = await createGroup(input)
            if (photo) {
                try {
                    await uploadGroupAvatar(group.id, photo)
                } catch (error) {
                    // группа уже создана - фото можно поставить позже в её настройках
                    notify({
                        icon: 'warn',
                        title: 'Фото группы не загрузилось',
                        description: `${errorMessage(error)}. Добавьте его позже в настройках группы`,
                    })
                }
            }
            return group
        },
        onSuccess: (group) => {
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            onCreated()
            navigate(chatPath(group.id))
        },
        onError: (error) => {
            // ошибку ссылки форма показывает у поля
            if (error instanceof ApiError && error.reason?.startsWith('SLUG_')) {
                return
            }
            notify({
                icon: 'warn',
                title: 'Не удалось создать чат',
                description: errorMessage(error),
            })
        },
    })
}
