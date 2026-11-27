import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { InfiniteData } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import { addReaction, removeReaction } from '../api/messages'
import { messagesKey, patchMessage } from './cache'
import type { MessagesPageParam } from './cache'
import type { MessageReaction, MessagesPage } from './types'

export interface ToggleReactionInput {
    messageId: string
    emoji: string
    active: boolean
}

function optimisticReactions(
    current: MessageReaction[] | undefined,
    emoji: string,
    active: boolean,
    myId: string | undefined,
): MessageReaction[] {
    const list = current ?? []
    const existing = list.find((r) => r.emoji === emoji)
    if (active) {
        if (existing) {
            if (existing.me) {
                return list
            }
            return list.map((r) =>
                r.emoji === emoji
                    ? {
                          ...r,
                          count: r.count + 1,
                          me: true,
                          user_ids:
                              myId && !r.user_ids.includes(myId)
                                  ? [...r.user_ids, myId]
                                  : r.user_ids,
                      }
                    : r,
            )
        }
        return [...list, { emoji, count: 1, me: true, user_ids: myId ? [myId] : [] }]
    }
    if (!existing || !existing.me) {
        return list
    }
    if (existing.count <= 1) {
        return list.filter((r) => r.emoji !== emoji)
    }
    return list.map((r) =>
        r.emoji === emoji
            ? {
                  ...r,
                  count: r.count - 1,
                  me: false,
                  user_ids: myId ? r.user_ids.filter((id) => id !== myId) : r.user_ids,
              }
            : r,
    )
}

export function useToggleReaction(conversationId: string, myId: string | undefined) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: ({ messageId, emoji, active }: ToggleReactionInput) =>
            active ? addReaction(messageId, emoji) : removeReaction(messageId, emoji),
        onMutate: ({ messageId, emoji, active }) => {
            const data = queryClient.getQueryData<InfiniteData<MessagesPage, MessagesPageParam>>(
                messagesKey(conversationId),
            )
            const msg = data?.pages.flatMap((p) => p.items).find((m) => m.id === messageId)
            const previous = msg?.reactions
            if (msg) {
                patchMessage(queryClient, conversationId, messageId, {
                    reactions: optimisticReactions(previous, emoji, active, myId),
                })
            }
            return { previous }
        },
        onSuccess: (reactions, { messageId }) => {
            patchMessage(queryClient, conversationId, messageId, { reactions })
        },
        onError: (error, { messageId }, context) => {
            if (context?.previous !== undefined) {
                patchMessage(queryClient, conversationId, messageId, {
                    reactions: context.previous,
                })
            }
            notify({
                icon: 'warn',
                title: 'Не удалось обновить реакцию',
                description: errorMessage(error),
            })
        },
    })
}
