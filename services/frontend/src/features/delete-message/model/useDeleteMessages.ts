import { useMutation, useQueryClient } from '@tanstack/react-query'

import { deleteMessages, patchMessage } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

export interface DeleteBatchInput {
    conversationId: string
    messageIds: string[]
}

// массовое мягкое удаление выбранных сообщений
export function useDeleteMessages() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: ({ messageIds }: DeleteBatchInput) => deleteMessages(messageIds),
        onSuccess: (_, { conversationId, messageIds }) => {
            for (const id of messageIds) {
                patchMessage(queryClient, conversationId, id, {
                    content: null,
                    entities: [],
                    is_deleted: true,
                })
            }
        },
        onError: (error) =>
            notify({
                icon: 'warn',
                title: 'Не удалось удалить сообщения',
                description: errorMessage(error),
            }),
    })
}
