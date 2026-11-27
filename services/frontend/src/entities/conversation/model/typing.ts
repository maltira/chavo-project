import { create } from 'zustand'

import { sendSocketMessage } from '@/shared/api'

// сколько держится индикатор «печатает…» без повторного сигнала
export const TYPING_TTL_MS = 5000
// минимальный интервал между исходящими сигналами typing из поля ввода
export const TYPING_SEND_INTERVAL_MS = 3000

const EMPTY_IDS: string[] = []

interface TypingState {
    // conversationId -> список активных userId (в порядке последнего сигнала)
    byConversation: Record<string, string[]>
    markTyping: (conversationId: string, userId: string) => void
    clearTyping: (conversationId: string, userId: string) => void
    clearAll: () => void
}

const timers = new Map<string, ReturnType<typeof setTimeout>>()

function timerKey(conversationId: string, userId: string) {
    return `${conversationId}:${userId}`
}

function clearTimer(key: string) {
    const existing = timers.get(key)
    if (existing !== undefined) {
        clearTimeout(existing)
        timers.delete(key)
    }
}

export const useTypingStore = create<TypingState>((set) => ({
    byConversation: {},
    markTyping: (conversationId, userId) => {
        const key = timerKey(conversationId, userId)
        clearTimer(key)
        timers.set(
            key,
            setTimeout(() => {
                timers.delete(key)
                useTypingStore.getState().clearTyping(conversationId, userId)
            }, TYPING_TTL_MS),
        )
        set((state) => {
            const current = state.byConversation[conversationId] ?? []
            if (current.includes(userId)) {
                return state
            }
            return {
                byConversation: {
                    ...state.byConversation,
                    [conversationId]: [...current, userId],
                },
            }
        })
    },
    clearTyping: (conversationId, userId) => {
        clearTimer(timerKey(conversationId, userId))
        set((state) => {
            const current = state.byConversation[conversationId]
            if (!current || !current.includes(userId)) {
                return state
            }
            const next = current.filter((id) => id !== userId)
            const copy = { ...state.byConversation }
            if (next.length === 0) {
                delete copy[conversationId]
            } else {
                copy[conversationId] = next
            }
            return { byConversation: copy }
        })
    },
    clearAll: () => {
        for (const timer of timers.values()) {
            clearTimeout(timer)
        }
        timers.clear()
        set({ byConversation: {} })
    },
}))

export function useTypingUsers(conversationId: string | undefined): string[] {
    return useTypingStore((state) =>
        conversationId ? (state.byConversation[conversationId] ?? EMPTY_IDS) : EMPTY_IDS,
    )
}

export function sendTyping(conversationId: string) {
    sendSocketMessage({ type: 'typing', conversation_id: conversationId })
}
