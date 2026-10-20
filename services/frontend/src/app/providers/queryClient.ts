import { QueryCache, QueryClient } from '@tanstack/react-query'

import { ApiError } from '@/shared/api'
import { notify } from '@/shared/lib'

// фоновые запросы падают молча, поэтому сеть показываем плашкой; запросы форм сами пишут ошибку у полей
export function handleQueryError(error: unknown) {
    if (error instanceof ApiError && error.reason === 'NETWORK_ERROR') {
        notify({
            id: 'network',
            icon: 'warn',
            title: 'Нет соединения',
            description: 'Проверьте интернет, мы попробуем снова',
        })
    }
}

export const queryClient = new QueryClient({
    queryCache: new QueryCache({ onError: handleQueryError }),
    defaultOptions: {
        queries: {
            staleTime: 30_000,
            refetchOnWindowFocus: false,
            retry: 1,
        },
    },
})
