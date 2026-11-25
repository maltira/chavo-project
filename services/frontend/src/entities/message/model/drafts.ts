import { create } from 'zustand'

import type { MessageEntity } from '@/shared/lib'

const STORAGE_KEY = 'chavo:drafts'

export interface DraftReply {
    id: string
    author: string
    text: string
}

export interface MessageDraft {
    text: string
    entities: MessageEntity[]
    replyTo?: DraftReply | null
}

function load(): Record<string, MessageDraft> {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (!raw) {
            return {}
        }
        const parsed = JSON.parse(raw) as unknown
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
            return {}
        }
        const result: Record<string, MessageDraft> = {}
        for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
            if (!value || typeof value !== 'object') {
                continue
            }
            const item = value as Partial<MessageDraft>
            const text = typeof item.text === 'string' ? item.text : ''
            const entities = Array.isArray(item.entities) ? item.entities : []
            const replyTo =
                item.replyTo &&
                typeof item.replyTo.id === 'string' &&
                typeof item.replyTo.author === 'string' &&
                typeof item.replyTo.text === 'string'
                    ? { id: item.replyTo.id, author: item.replyTo.author, text: item.replyTo.text }
                    : null
            if (text.trim() !== '' || replyTo) {
                result[key] = { text, entities, replyTo }
            }
        }
        return result
    } catch {
        return {}
    }
}

function save(items: Record<string, MessageDraft>) {
    try {
        if (Object.keys(items).length === 0) {
            localStorage.removeItem(STORAGE_KEY)
        } else {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
        }
    } catch {
        // если хранилище недоступно, черновики останутся хотя бы в памяти вкладки
    }
}

interface DraftsState {
    items: Record<string, MessageDraft>
    setDraft: (chatKey: string, draft: MessageDraft | null) => void
    clearDraft: (chatKey: string) => void
    clearAll: () => void
}

export const useDrafts = create<DraftsState>((set) => ({
    items: load(),
    setDraft: (chatKey, draft) =>
        set((state) => {
            const empty = !draft || (draft.text.trim() === '' && !draft.replyTo)
            if (empty) {
                if (!(chatKey in state.items)) {
                    return state
                }
                const next = { ...state.items }
                delete next[chatKey]
                save(next)
                return { items: next }
            }
            const next = {
                ...state.items,
                [chatKey]: {
                    text: draft.text,
                    entities: draft.entities,
                    replyTo: draft.replyTo ?? null,
                },
            }
            save(next)
            return { items: next }
        }),
    clearDraft: (chatKey) =>
        set((state) => {
            if (!(chatKey in state.items)) {
                return state
            }
            const next = { ...state.items }
            delete next[chatKey]
            save(next)
            return { items: next }
        }),
    clearAll: () =>
        set(() => {
            save({})
            return { items: {} }
        }),
}))

export function getDraft(chatKey: string | undefined): MessageDraft | undefined {
    return chatKey ? useDrafts.getState().items[chatKey] : undefined
}

export function useDraft(chatKey: string | undefined): MessageDraft | undefined {
    return useDrafts((state) => (chatKey ? state.items[chatKey] : undefined))
}
