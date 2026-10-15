import { create } from 'zustand'

const STORAGE_KEY = 'chavo:recent-users'
// сколько профилей помнит блок «Недавние»
export const RECENT_LIMIT = 10

// localStorage может быть недоступен (приватный режим, запрет): тогда список живёт только в памяти
function load(): string[] {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        const parsed: unknown = raw ? JSON.parse(raw) : []
        return Array.isArray(parsed)
            ? parsed.filter((id): id is string => typeof id === 'string').slice(0, RECENT_LIMIT)
            : []
    } catch {
        return []
    }
}

function save(ids: string[]) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(ids))
    } catch {
        // не сохранилось - останется до перезагрузки
    }
}

interface RecentUsersState {
    ids: string[]
    add: (userId: string) => void
    clear: () => void
}

// профили, которые недавно открывали: свежие первыми, без повторов
export const useRecentUsers = create<RecentUsersState>()((set, get) => ({
    ids: load(),
    add: (userId) => {
        const ids = [userId, ...get().ids.filter((id) => id !== userId)].slice(0, RECENT_LIMIT)
        save(ids)
        set({ ids })
    },
    clear: () => {
        save([])
        set({ ids: [] })
    },
}))
