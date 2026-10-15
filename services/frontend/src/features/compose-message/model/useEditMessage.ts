import { useMutation, useQueryClient } from '@tanstack/react-query'

import { editMessage, patchMessage } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

export function useEditMessage() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: ({ id, content }: { id: string; content: string }) => editMessage(id, content),
        onSuccess: (message) => {
            patchMessage(queryClient, message.conversation_id, message.id, message)
        },
        onError: (error) =>
            notify({
                icon: 'warn',
                title: 'Не удалось изменить сообщение',
                description: errorMessage(error),
            }),
    })
}
