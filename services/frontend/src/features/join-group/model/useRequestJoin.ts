import { useMutation } from '@tanstack/react-query'

import { requestJoin, useGroupCard } from '@/entities/conversation'
import { ApiError, errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

// заявка в закрытую группу по коду ссылки; решение принимает админ, о нём придёт событие
export function useRequestJoin() {
    const markSent = () => {
        const { target, openPreview } = useGroupCard.getState()
        if (target?.kind === 'preview') {
            openPreview({ ...target.group, request_status: 'pending' })
        }
    }

    return useMutation({
        mutationFn: ({ groupId, code }: { groupId: string; code: string }) =>
            requestJoin(groupId, code),
        onSuccess: () => {
            markSent()
            notify({
                icon: 'check',
                title: 'Запрос отправлен',
                description: 'Чат появится в списке, когда администратор одобрит заявку',
            })
        },
        onError: (error) => {
            // заявка уже есть (например, с другого устройства) - показываем как отправленную
            if (error instanceof ApiError && error.reason === 'JOIN_REQUEST_EXISTS') {
                markSent()
                return
            }
            notify({
                icon: 'warn',
                title: 'Не удалось отправить запрос',
                description: errorMessage(error),
            })
        },
    })
}
