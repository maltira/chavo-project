import { useMutation, useQueryClient } from '@tanstack/react-query'

import { editMessage, patchMessage } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import type { MessageEntity } from '@/shared/lib'

export function useEditMessage() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: ({
            id,
            content,
            entities,
            disable_link_preview,
        }: {
            id: string
            content: string
            entities?: MessageEntity[]
            disable_link_preview?: boolean
        }) => editMessage(id, content, entities, disable_link_preview),
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
