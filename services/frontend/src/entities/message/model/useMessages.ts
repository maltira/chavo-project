import { useInfiniteQuery } from '@tanstack/react-query'

import { getMessages } from '../api/messages'
import { messagesKey } from './cache'

export function useMessages(conversationId: string) {
    const query = useInfiniteQuery({
        queryKey: messagesKey(conversationId),
        queryFn: ({ pageParam }) => getMessages(conversationId, pageParam),
        initialPageParam: undefined as string | undefined,
        getNextPageParam: (last) => last.next_before ?? undefined,
        // события дописывают историю сами; перечитывание при каждом открытии не нужно
        staleTime: Infinity,
    })

    return {
        ...query,
        data: query.data?.pages.flatMap((page) => page.items),
    }
}
