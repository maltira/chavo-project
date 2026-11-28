import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'

import * as api from '../api/messages'
import { messagesKey } from './cache'
import type { Message } from './types'
import { useToggleReaction } from './useReactions'

const baseMessage: Message = {
    id: 'm1',
    conversation_id: 'c1',
    sender_id: 'other',
    content: 'Привет',
    reply_to_message_id: null,
    is_edited: false,
    is_deleted: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: null,
    reactions: [],
}

describe('useToggleReaction', () => {
    afterEach(() => {
        vi.restoreAllMocks()
        useNotifications.setState({ items: [] })
    })

    function setup() {
        const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
        client.setQueryData(messagesKey('c1'), {
            pages: [{ items: [baseMessage], has_more: false }],
            pageParams: [undefined],
        })
        const wrapper = ({ children }: { children: ReactNode }) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        )
        return { client, wrapper }
    }

    it('suppresses notification and rolls back when REACTION_LIMIT is returned', async () => {
        vi.spyOn(api, 'addReaction').mockRejectedValue(
            new ApiError(400, 'Достигнут лимит реакций на сообщение', 'REACTION_LIMIT'),
        )
        const { client, wrapper } = setup()
        const { result } = renderHook(() => useToggleReaction('c1', 'me'), { wrapper })

        act(() => {
            result.current.mutate({ messageId: 'm1', emoji: '🔥', active: true })
        })

        await waitFor(() => expect(result.current.isError).toBe(true))
        expect(useNotifications.getState().items).toHaveLength(0)
        const cached = client.getQueryData<{ pages: { items: Message[] }[] }>(messagesKey('c1'))
        expect(cached?.pages[0]?.items[0]?.reactions).toEqual([])
    })

    it('shows notification on other reaction errors', async () => {
        vi.spyOn(api, 'addReaction').mockRejectedValue(
            new ApiError(500, 'Ошибка сервера', 'INTERNAL'),
        )
        const { wrapper } = setup()
        const { result } = renderHook(() => useToggleReaction('c1', 'me'), { wrapper })

        act(() => {
            result.current.mutate({ messageId: 'm1', emoji: '🔥', active: true })
        })

        await waitFor(() => expect(result.current.isError).toBe(true))
        expect(useNotifications.getState().items).toHaveLength(1)
        expect(useNotifications.getState().items[0]?.title).toBe('Не удалось обновить реакцию')
    })
})
