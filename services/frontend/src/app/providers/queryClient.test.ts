import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'

import { handleQueryError } from './queryClient'

describe('handleQueryError', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        useNotifications.setState({ items: [] })
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('shows one notification for network failures', () => {
        handleQueryError(new ApiError(0, 'x', 'NETWORK_ERROR'))
        handleQueryError(new ApiError(0, 'x', 'NETWORK_ERROR'))

        const items = useNotifications.getState().items
        expect(items).toHaveLength(1)
        expect(items[0]?.title).toBe('Нет соединения')
    })

    it('ignores other errors', () => {
        handleQueryError(new ApiError(400, 'x', 'INVALID_OTP'))
        handleQueryError(new Error('boom'))
        expect(useNotifications.getState().items).toHaveLength(0)
    })
})
