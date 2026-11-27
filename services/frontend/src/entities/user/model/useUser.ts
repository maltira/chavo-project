import { useQueries, useQuery } from '@tanstack/react-query'

import { getUser } from '../api/user'
import type { Profile } from './types'

export const userKey = (userId: string) => ['user', userId] as const

// профиль собеседника
export function useUser(userId: string | null | undefined) {
    return useQuery({
        queryKey: userKey(userId ?? ''),
        queryFn: () => getUser(userId!),
        enabled: !!userId,
        staleTime: 5 * 60_000,
    })
}

// пакетная загрузка профилей с переиспользованием общего кеша userKey
export function useUsers(userIds: string[]): Profile[] {
    const results = useQueries({
        queries: userIds.map((id) => ({
            queryKey: userKey(id),
            queryFn: () => getUser(id),
            staleTime: 5 * 60_000,
        })),
    })
    const out: Profile[] = []
    for (const r of results) {
        if (r.data) {
            out.push(r.data)
        }
    }
    return out
}
