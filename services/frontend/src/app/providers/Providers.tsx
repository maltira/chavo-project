import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'

import { router } from '../lib/router'
import { NotificationsHost } from './NotificationsHost'
import { queryClient } from './queryClient'

export function Providers() {
    return (
        <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
            <NotificationsHost />
        </QueryClientProvider>
    )
}
