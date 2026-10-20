import { useInfiniteQuery } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import { getBans, getJoinRequests, getMembersPage, GROUP_PAGE } from '../api/groups'
import { conversationKey, membersKey } from './useConversation'
import { conversationsKey } from './useConversations'

export const groupMembersKey = (id: string) => ['group-members', id] as const
export const joinRequestsKey = (id: string) => ['join-requests', id] as const
export const bansKey = (id: string) => ['bans', id] as const

function usePaged<T>(
    queryKey: readonly unknown[],
    load: (offset: number) => Promise<T[]>,
    enabled: boolean,
) {
    const query = useInfiniteQuery({
        queryKey,
        queryFn: ({ pageParam }) => load(pageParam),
        initialPageParam: 0,
        getNextPageParam: (last: T[], all: T[][]) =>
            last.length < GROUP_PAGE ? undefined : all.flat().length,
        enabled,
    })
    return { ...query, data: query.data?.pages.flat() }
}

// участники группы постранично (окно «Участники»); курсоры прочтения - в useMembers
export function useGroupMembers(id: string, enabled = true) {
    return usePaged(groupMembersKey(id), (offset) => getMembersPage(id, offset), enabled)
}

// заявки на рассмотрении; видны только админу
export function useJoinRequests(id: string, enabled = true) {
    return usePaged(joinRequestsKey(id), (offset) => getJoinRequests(id, offset), enabled)
}

// заблокированные в группе; видны только админу
export function useBans(id: string, enabled = true) {
    return usePaged(bansKey(id), (offset) => getBans(id, offset), enabled)
}

// после изменения состава или настроек группы: карточка, список чатов и списки окна группы
export function invalidateGroup(queryClient: QueryClient, id: string) {
    for (const queryKey of [
        conversationKey(id),
        membersKey(id),
        groupMembersKey(id),
        joinRequestsKey(id),
        bansKey(id),
        conversationsKey,
    ]) {
        void queryClient.invalidateQueries({ queryKey })
    }
}
