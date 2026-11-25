import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router'

import { conversationsKey, deleteConversation } from '@/entities/conversation'
import { messagesKey } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

interface LeaveInput {
    conversationId: string
    forEveryone?: boolean
}

// выход из группы (участник), её удаление (админ) или удаление личного чата (у себя или у обоих):
// чат пропадает из списка, открытый - закрывается
export function useLeaveChat() {
    const queryClient = useQueryClient()
    const navigate = useNavigate()
    const location = useLocation()

    return useMutation({
        mutationFn: ({ conversationId, forEveryone = false }: LeaveInput) =>
            deleteConversation(conversationId, forEveryone),
        onSuccess: (_, { conversationId }) => {
            queryClient.removeQueries({ queryKey: messagesKey(conversationId) })
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            if (location.pathname === chatPath(conversationId)) {
                navigate('/', { replace: true })
            }
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })
}
