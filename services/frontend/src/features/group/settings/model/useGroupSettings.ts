import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'

import {
    conversationsKey,
    deleteConversation,
    invalidateGroup,
    updateGroup,
} from '@/entities/conversation'
import type { GroupUpdate } from '@/entities/conversation'
import { ApiError, errorMessage } from '@/shared/api'
import { ROUTES } from '@/shared/config'
import { notify } from '@/shared/lib'

// ошибки кода ссылки показываются у поля, остальные - уведомлением
const SLUG_REASONS = new Set(['SLUG_TAKEN', 'SLUG_INVALID', 'SLUG_REQUIRED'])

export function isSlugError(error: unknown): error is ApiError {
    return error instanceof ApiError && !!error.reason && SLUG_REASONS.has(error.reason)
}

// сохранение настроек группы; карточку и список перечитываем - в ответе нет счётчиков
export function useUpdateGroup(id: string, onSaved: () => void) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (patch: GroupUpdate) => updateGroup(id, patch),
        onSuccess: () => {
            invalidateGroup(queryClient, id)
            notify({ icon: 'check', title: 'Изменения сохранены' })
            onSaved()
        },
        onError: (error) => {
            if (!isSlugError(error)) {
                notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })
            }
        },
    })
}

// админ удаляет группу у всех; окно закрывается, открытый чат пропадает
export function useDeleteGroup(id: string, onDeleted: () => void) {
    const queryClient = useQueryClient()
    const navigate = useNavigate()
    return useMutation({
        mutationFn: () => deleteConversation(id),
        onSuccess: () => {
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            onDeleted()
            navigate(ROUTES.home)
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не удалось удалить', description: errorMessage(error) }),
    })
}
