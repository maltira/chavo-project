import { keepPreviousData, useQuery } from '@tanstack/react-query'

import { searchUsers } from '../api/user'

// результаты поиска людей; пока печатают, на экране остаются прежние
export function useUserSearch(query: string) {
    return useQuery({
        queryKey: ['users-search', query],
        queryFn: () => searchUsers(query),
        enabled: query.length > 0,
        placeholderData: keepPreviousData,
        staleTime: 30_000,
    })
}
