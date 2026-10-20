import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
    clearGroupStatus,
    conversationKey,
    conversationsKey,
    setGroupStatus,
    useConversation,
} from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { clearMyStatus, setMyStatus, useMe } from '@/entities/user'
import type { Profile, Status, StatusInput } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import type { CustomTarget } from './target'

// текущий статус цели: мой - из профиля, группы - из карточки чата
export function useTargetStatus(target: CustomTarget): Status | null {
    const me = useMe()
    const groupId = target.kind === 'group' ? target.id : ''
    const chat = useConversation(groupId, target.kind === 'group')
    return (target.kind === 'me' ? me.data?.status : chat.data?.status) ?? null
}

// установка и снятие статуса; кеш обновляется сразу, событий об этом нет
export function useStatusMutations(target: CustomTarget, onDone: () => void) {
    const queryClient = useQueryClient()
    const putStatus = (status: Status | null) => {
        if (target.kind === 'me') {
            queryClient.setQueryData<Profile | null>(['me'], (me) => (me ? { ...me, status } : me))
            return
        }
        queryClient.setQueryData<ConversationSummary>(conversationKey(target.id), (chat) =>
            chat ? { ...chat, status } : chat,
        )
        void queryClient.invalidateQueries({ queryKey: conversationsKey })
    }
    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })

    const set = useMutation({
        mutationFn: (input: StatusInput) =>
            target.kind === 'me' ? setMyStatus(input) : setGroupStatus(target.id, input),
        onSuccess: (status) => {
            putStatus(status)
            onDone()
        },
        onError,
    })
    const clear = useMutation({
        mutationFn: () => (target.kind === 'me' ? clearMyStatus() : clearGroupStatus(target.id)),
        onSuccess: () => {
            putStatus(null)
            onDone()
        },
        onError,
    })

    return { set, clear, pending: set.isPending || clear.isPending }
}
