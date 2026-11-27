import { useQuery } from '@tanstack/react-query'

import { getTwoFactor } from '../api/twoFactor'

export const twoFactorKey = ['two-factor'] as const

export function useTwoFactor(enabled = true) {
    return useQuery({
        queryKey: twoFactorKey,
        queryFn: getTwoFactor,
        enabled,
    })
}
