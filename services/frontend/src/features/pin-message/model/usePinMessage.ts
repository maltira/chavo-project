import { useMutation, useQueryClient } from '@tanstack/react-query'

import { pinMessage, pinsKey, unpinMessage } from '@/entities/message'
import type { PinScope } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

interface PinChange {
    messageId: string
    scope: PinScope
    pinned: boolean
}

// закреп и открепление сообщения; список закрепов перечитывается, у остальных - по событию
export function usePinMessage(chatId: string) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async ({ messageId, scope, pinned }: PinChange) => {
            if (pinned) {
                await pinMessage(chatId, messageId, scope)
            } else {
                await unpinMessage(chatId, messageId, scope)
            }
        },
        onSuccess: () => void queryClient.invalidateQueries({ queryKey: pinsKey(chatId) }),
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })
}
