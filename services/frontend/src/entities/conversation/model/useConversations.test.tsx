import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'

import { conversationsKey, useConversations } from './useConversations'
import type { ConversationSummary } from './types'

function chat(id: string): ConversationSummary {
    const at = '2026-10-14T10:00:00Z'
    return {
        id,
        conversation_type: 'group',
        visibility: 'private',
        name: id,
        description: null,
        avatar_url: null,
        last_message_at: at,
        created_at: at,
        updated_at: at,
        status: null,
        avatar_shape: null,
        my_role: 'member',
        peer_id: null,
        last_message: null,
        unread_count: 0,
        last_message_read: false,
    }
}

const page = (from: number, n: number) => Array.from({ length: n }, (_, i) => chat(`c${from + i}`))

function setup() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    return { client, wrapper }
}

describe('useConversations', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('loads the next page by offset until a partial page', async () => {
        const urls: string[] = []
        vi.stubGlobal(
            'fetch',
            vi.fn((url: string) => {
                urls.push(url)
                const offset = Number(new URL(url, 'http://x').searchParams.get('offset'))
                const items = offset === 0 ? page(0, 30) : page(30, 5)
                return Promise.resolve(new Response(JSON.stringify({ items, limit: 30, offset })))
            }),
        )
        const { wrapper } = setup()
        const { result } = renderHook(() => useConversations(), { wrapper })

        await waitFor(() => expect(result.current.data).toHaveLength(30))
        expect(result.current.hasNextPage).toBe(true)

        await result.current.fetchNextPage()
        await waitFor(() => expect(result.current.data).toHaveLength(35))
        expect(result.current.hasNextPage).toBe(false)
        expect(urls[1]).toContain('offset=30')
    })

    it('keeps each chat once when an event moved it between pages', async () => {
        const { client, wrapper } = setup()
        // страницы пересеклись: чат c29 уже есть и на первой, и на второй
        client.setQueryData(conversationsKey, {
            pages: [page(0, 30), [chat('c29'), chat('c30')]],
            pageParams: [0, 30],
        })
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
        const { result } = renderHook(() => useConversations(), { wrapper })
        await waitFor(() => expect(result.current.data).toHaveLength(31))
    })
})
