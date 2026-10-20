import { renderHook } from '@testing-library/react'

import { useMarkRead } from './useMarkRead'

const fetchMock = vi.fn()

const reads = () =>
    fetchMock.mock.calls
        .filter(([url]) => String(url).endsWith('/read'))
        .map(([, init]) => JSON.parse(init.body as string).message_id as string)

describe('useMarkRead', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    it('merges quick reads into one request with the freshest message', () => {
        const { rerender } = renderHook(({ id }) => useMarkRead('c1', id), {
            initialProps: { id: 'm1' as string | undefined },
        })
        rerender({ id: 'm2' })
        vi.advanceTimersByTime(200)
        rerender({ id: 'm3' })
        expect(reads()).toEqual([])

        vi.advanceTimersByTime(400)
        expect(reads()).toEqual(['m3'])
    })

    it('does not repeat the same message and skips temporary ones', () => {
        const { rerender } = renderHook(({ id }) => useMarkRead('c1', id), {
            initialProps: { id: 'm1' as string | undefined },
        })
        vi.advanceTimersByTime(400)
        rerender({ id: 'm1' })
        rerender({ id: 'temp-1' })
        vi.advanceTimersByTime(400)
        expect(reads()).toEqual(['m1'])
    })

    it('sends the pending read right away when leaving the chat', () => {
        const { unmount } = renderHook(() => useMarkRead('c1', 'm5'))
        expect(reads()).toEqual([])
        unmount()
        expect(reads()).toEqual(['m5'])
    })
})
