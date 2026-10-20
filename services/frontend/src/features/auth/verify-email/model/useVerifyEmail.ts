import { useQuery } from '@tanstack/react-query'

import { verifyEmail } from '../api/verify'

export function useVerifyEmail(token: string | null) {
    return useQuery({
        queryKey: ['verify-email', token],
        queryFn: () => verifyEmail(token!),
        enabled: !!token,
        retry: false,
        staleTime: Infinity,
        gcTime: Infinity,
    })
}
