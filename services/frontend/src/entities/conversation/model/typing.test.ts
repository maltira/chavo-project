import { TYPING_TTL_MS, useTypingStore } from './typing'

describe('useTypingStore', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        useTypingStore.getState().clearAll()
    })

    afterEach(() => {
        useTypingStore.getState().clearAll()
        vi.useRealTimers()
    })

    it('tracks typing users per conversation and expires them after TTL', () => {
        const store = useTypingStore.getState()
        store.markTyping('c1', 'u1')
        store.markTyping('c1', 'u2')
        expect(useTypingStore.getState().byConversation.c1).toEqual(['u1', 'u2'])

        vi.advanceTimersByTime(TYPING_TTL_MS - 1000)
        // повторный сигнал продлевает таймер только для u1
        useTypingStore.getState().markTyping('c1', 'u1')
        vi.advanceTimersByTime(1000)
        expect(useTypingStore.getState().byConversation.c1).toEqual(['u1'])

        vi.advanceTimersByTime(TYPING_TTL_MS)
        expect(useTypingStore.getState().byConversation.c1).toBeUndefined()
    })

    it('clears typing user immediately on clearTyping', () => {
        useTypingStore.getState().markTyping('c1', 'u1')
        useTypingStore.getState().clearTyping('c1', 'u1')
        expect(useTypingStore.getState().byConversation.c1).toBeUndefined()
    })
})
