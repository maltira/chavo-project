import { useQuery } from '@tanstack/react-query'

import { getSessions } from '../api/sessions'

export const sessionsKey = ['sessions'] as const

// мои активные сеансы на всех устройствах
export function useSessions(enabled = true) {
    return useQuery({ queryKey: sessionsKey, queryFn: getSessions, enabled })
}
