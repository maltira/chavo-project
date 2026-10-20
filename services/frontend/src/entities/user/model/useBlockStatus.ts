import { useQuery } from '@tanstack/react-query'

import { getBlockStatus } from '../api/user'

export const blockStatusKey = (userId: string) => ['block-status', userId] as const

// кто кого заблокировал; меняется событиями user.blocked/unblocked
export function useBlockStatus(userId: string | null | undefined) {
    return useQuery({
        queryKey: blockStatusKey(userId ?? ''),
        queryFn: () => getBlockStatus(userId!),
        enabled: !!userId,
    })
}
