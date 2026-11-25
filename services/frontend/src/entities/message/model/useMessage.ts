import { useQuery } from '@tanstack/react-query'

import { getMessage } from '../api/messages'

export const messageKey = (id: string) => ['message', id] as const

// одно сообщение, когда его нет в загруженной истории (например, закреплённое из строки «закрепил»
// в списке чатов); правки подтянутся при следующем открытии
export function useMessage(id: string | null | undefined) {
    return useQuery({
        queryKey: messageKey(id ?? ''),
        queryFn: () => getMessage(id!),
        enabled: !!id,
        staleTime: 60_000,
        retry: false,
    })
}
