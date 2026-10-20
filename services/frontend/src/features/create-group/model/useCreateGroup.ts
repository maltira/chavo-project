import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'

import { conversationsKey, createGroup } from '@/entities/conversation'
import type { CreateGroupInput } from '@/entities/conversation'
import { ApiError, errorMessage } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

// создание группы: после него группа сразу открывается; при ошибке форма остаётся как была
export function useCreateGroup(onCreated: () => void) {
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    return useMutation({
        mutationFn: (input: CreateGroupInput) => createGroup(input),
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
