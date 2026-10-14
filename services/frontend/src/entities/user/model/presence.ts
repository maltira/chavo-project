import { useQuery } from '@tanstack/react-query'

export const presenceKey = (userId: string) => ['presence', userId] as const

// онлайн из событий WebSocket; пока событий не было, верен онлайн из профиля
export interface Presence {
    online: boolean
    lastSeenAt: string | null
}

// кеш наполняет только мост событий, поэтому запрос сам ничего не загружает
export function usePresence(userId: string | null | undefined) {
    return useQuery<Presence | null>({
        queryKey: presenceKey(userId ?? ''),
        queryFn: () => null,
        enabled: false,
    }).data
}
