import { useQuery } from '@tanstack/react-query'

import { getUser } from '../api/user'

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
