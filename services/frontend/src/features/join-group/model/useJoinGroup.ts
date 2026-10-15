import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'

import { conversationsKey, joinGroup } from '@/entities/conversation'
import { errorMessage } from '@/shared/api'
import { chatPath } from '@/shared/config'
import { notify } from '@/shared/lib'

// вступление в открытую группу: после него группа появляется в списке и сразу открывается
export function useJoinGroup(onJoined?: () => void) {
    const queryClient = useQueryClient()
    const navigate = useNavigate()

    return useMutation({
        mutationFn: (groupId: string) => joinGroup(groupId),
        onSuccess: (_, groupId) => {
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            onJoined?.()
            navigate(chatPath(groupId))
        },
        onError: (error) =>
            notify({
                icon: 'warn',
                title: 'Не удалось вступить',
                description: errorMessage(error),
            }),
    })
}
