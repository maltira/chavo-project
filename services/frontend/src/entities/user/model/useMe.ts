import { useQuery } from '@tanstack/react-query'

import { getMe } from '../api/me'

// свой профиль; data === null - профиль ещё не заполнен
export function useMe(enabled = true) {
    return useQuery({ queryKey: ['me'], queryFn: getMe, enabled })
}
