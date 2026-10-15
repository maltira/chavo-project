import { keepPreviousData, useQuery } from '@tanstack/react-query'

import { searchGroups } from '../api/conversations'

// открытые группы по названию; пока печатают, на экране остаются прежние результаты
export function useGroupSearch(query: string) {
    return useQuery({
        queryKey: ['groups-search', query],
        queryFn: () => searchGroups(query),
        enabled: query.length > 0,
        placeholderData: keepPreviousData,
        staleTime: 30_000,
    })
}
