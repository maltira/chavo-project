import { useQuery } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import type { ServerEvent } from '@/shared/api'

import { getPins } from '../api/pins'
import { messageKey } from './useMessage'

export const pinsKey = (chatId: string) => ['pins', chatId] as const

// закреплённые сообщения чата: общие и мои личные
export function usePins(chatId: string) {
    return useQuery({ queryKey: pinsKey(chatId), queryFn: () => getPins(chatId) })
}

// закрепы меняются событием conversation.pins.changed; правка сообщения меняет текст закрепа
// и строки «закрепил сообщение»
export function applyPinsEvent(queryClient: QueryClient, event: ServerEvent) {
    if (event.type !== 'conversation.pins.changed' && event.type !== 'message.updated') {
        return
    }
    const data = (event.data ?? {}) as { conversation_id?: string; message_id?: string }
    if (data.conversation_id) {
        void queryClient.invalidateQueries({ queryKey: pinsKey(data.conversation_id) })
    }
    if (event.type === 'message.updated' && data.message_id) {
        void queryClient.invalidateQueries({ queryKey: messageKey(data.message_id) })
    }
}
