import { useMutation, useQueryClient } from '@tanstack/react-query'

import { deleteMessage, patchMessage } from '@/entities/message'
import type { Message } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

// удаление мягкое: сообщение остаётся в истории с пометкой «удалено»
export function useDeleteMessage() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: (message: Message) => deleteMessage(message.id),
        onSuccess: (_, message) => {
            patchMessage(queryClient, message.conversation_id, message.id, {
                content: null,
                is_deleted: true,
            })
        },
        onError: (error) =>
            notify({
                icon: 'warn',
                title: 'Не удалось удалить сообщение',
                description: errorMessage(error),
            }),
    })
}
