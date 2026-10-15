import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router'

import { conversationsKey, deleteConversation } from '@/entities/conversation'
import { messagesKey } from '@/entities/message'
import { errorMessage } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

// выход из группы (участник) или её удаление (админ): чат пропадает из списка, открытый - закрывается
export function useLeaveChat() {
    const queryClient = useQueryClient()
    const navigate = useNavigate()
    const location = useLocation()

    return useMutation({
        mutationFn: (conversationId: string) => deleteConversation(conversationId),
        onSuccess: (_, conversationId) => {
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
