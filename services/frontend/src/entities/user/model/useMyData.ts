import { useInfiniteQuery, useQuery } from '@tanstack/react-query'

import { BLOCKED_PAGE, getBlocked, getSettings } from '../api/user'

export const settingsKey = ['settings'] as const
export const blockedKey = ['blocked'] as const

// мои настройки конфиденциальности
export function useSettings(enabled = true) {
    return useQuery({ queryKey: settingsKey, queryFn: getSettings, enabled })
}

// мой чёрный список страницами: неполная страница - конец списка
export function useBlocked(enabled = true) {
    const query = useInfiniteQuery({
        queryKey: blockedKey,
        queryFn: ({ pageParam }) => getBlocked(pageParam),
        initialPageParam: 0,
        getNextPageParam: (last, all) =>
            last.length < BLOCKED_PAGE ? undefined : all.flat().length,
        enabled,
    })

    return { ...query, data: query.data?.pages.flat() }
}
