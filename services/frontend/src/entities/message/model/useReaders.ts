import { useQuery } from '@tanstack/react-query'

import { getMessageReaders } from '../api/messages'

export const readersKey = (conversationId: string, messageId: string) =>
    ['readers', conversationId, messageId] as const

// список прочитавших сообщение в группе; запрашивается заново при каждом открытии окна
export function useReaders(
    conversationId: string,
    messageId: string | null | undefined,
    enabled = true,
) {
    return useQuery({
        queryKey: readersKey(conversationId, messageId ?? ''),
        queryFn: () => getMessageReaders(conversationId, messageId!),
        enabled: enabled && !!messageId,
        staleTime: 0,
    })
}
