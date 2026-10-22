import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
    muteChat,
    setChatArchived,
    setChatPinned,
    setChatSettings,
    unmuteChat,
} from '@/entities/conversation'
import type { ChatSettingsResponse } from '@/entities/conversation'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

type Change =
    | { kind: 'mute'; duration: string }
    | { kind: 'unmute' }
    | { kind: 'archive'; archived: boolean }
    | { kind: 'pin'; pinned: boolean }

function send(id: string, change: Change): Promise<ChatSettingsResponse> {
    switch (change.kind) {
        case 'mute':
            return muteChat(id, change.duration)
        case 'unmute':
            return unmuteChat(id)
        case 'archive':
            return setChatArchived(id, change.archived)
        case 'pin':
            return setChatPinned(id, change.pinned)
    }
}

// личные настройки чата: «без звука», архив, закреп; кеш меняется по ответу, другие вкладки - по событию
export function useChatSettings(chatId: string) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: (change: Change) => send(chatId, change),
        onSuccess: (settings, change) => {
            setChatSettings(queryClient, chatId, settings)
            if (change.kind === 'archive') {
                notify({
                    id: `archive-${chatId}`,
                    icon: 'shape-check',
                    title: change.archived ? 'Чат в архиве' : 'Чат вернулся в список',
                })
            }
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })
}
