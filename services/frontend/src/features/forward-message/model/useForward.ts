import { useMutation, useQueryClient } from '@tanstack/react-query'

import { conversationsKey } from '@/entities/conversation'
import { forwardMessages } from '@/entities/message'
import type { ForwardInput } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { notify, plural } from '@/shared/lib'

// пересылка: копии приходят в чаты событиями; список перечитывается - могло появиться «Избранное»
export function useForward(onDone: () => void) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (input: ForwardInput) => forwardMessages(input),
        onSuccess: (chats) => {
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            notify({
                id: 'forwarded',
                icon: 'shape-check',
                title: `Переслано в ${chats.length} ${plural(chats.length, ['чат', 'чата', 'чатов'])}`,
            })
            onDone()
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })
}
