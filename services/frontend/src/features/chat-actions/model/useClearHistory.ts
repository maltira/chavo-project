import { useMutation, useQueryClient } from '@tanstack/react-query'

import { applyConversationEvent, clearHistory } from '@/entities/conversation'
import { clearMessagesCache, pinsKey } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

// очистка истории чата только для себя: сам чат и его личные настройки остаются в списке
export function useClearHistory() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: (conversationId: string) => clearHistory(conversationId),
        onSuccess: (_, conversationId) => {
            clearMessagesCache(queryClient, conversationId)
            applyConversationEvent(
                queryClient,
                {
                    type: 'conversation.history.cleared',
                    data: { conversation_id: conversationId },
                },
                undefined,
            )
            void queryClient.invalidateQueries({ queryKey: pinsKey(conversationId) })
            notify({ id: 'history-cleared', icon: 'check', title: 'История очищена' })
        },
        onError: (error) =>
            notify({
                icon: 'warn',
                title: 'Не удалось очистить историю',
                description: errorMessage(error),
            }),
    })
}
