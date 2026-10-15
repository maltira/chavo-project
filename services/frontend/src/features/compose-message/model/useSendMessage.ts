import { useMutation, useQueryClient } from '@tanstack/react-query'

import { addMessage, removeMessage, replaceMessage, sendMessage } from '@/entities/message'
import type { Message, SendMessageInput } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

let tempCounter = 0

// отправка с оптимистичной вставкой: сообщение видно сразу, при ошибке исчезает
export function useSendMessage(myId: string | undefined) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: (input: SendMessageInput) => sendMessage(input),
        onMutate: (input) => {
            const temp: Message = {
                id: `temp-${++tempCounter}`,
                conversation_id: input.conversation_id,
                sender_id: myId ?? '',
                content: input.content,
                reply_to_message_id: input.reply_to_message_id ?? null,
                is_edited: false,
                is_deleted: false,
                created_at: new Date().toISOString(),
                updated_at: null,
                pending: true,
            }
            addMessage(queryClient, temp)
            return { tempId: temp.id }
        },
        onSuccess: (message, _input, context) => {
            replaceMessage(queryClient, context.tempId, message)
        },
        onError: (error, input, context) => {
            if (context) {
                removeMessage(queryClient, input.conversation_id, context.tempId)
            }
            notify({
                icon: 'warn',
                title: 'Сообщение не отправлено',
                description: errorMessage(error),
            })
        },
    })
}
