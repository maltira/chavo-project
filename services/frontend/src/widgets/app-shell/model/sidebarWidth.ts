import { useCallback, useState } from 'react'

export const DEFAULT_WIDTH = 362
export const COMPACT_WIDTH = 76
export const MAX_WIDTH = 450
export const COMPACT_THRESHOLD = 240

const STORAGE_KEY = 'chavo:sidebar-width'

export function normalizeWidth(raw: number): number {
    if (!Number.isFinite(raw)) {
        return DEFAULT_WIDTH
    }
    if (raw < COMPACT_THRESHOLD) {
        return COMPACT_WIDTH
    }
    return Math.min(MAX_WIDTH, Math.round(raw))
}

export function isCompact(width: number) {
    return width <= COMPACT_WIDTH
}

// localStorage может быть недоступен (приватный режим, запрет): тогда живёт только в памяти
export function loadWidth(): number {
    try {
        const stored = localStorage.getItem(STORAGE_KEY)
        return stored === null ? DEFAULT_WIDTH : normalizeWidth(Number(stored))
    } catch {
        return DEFAULT_WIDTH
    }
}

function saveWidth(width: number) {
    try {
        localStorage.setItem(STORAGE_KEY, String(width))
    } catch {
        // ширина просто не запомнится
    }
}

export function useSidebarWidth() {
    const [width, setWidth] = useState(loadWidth)

    // во время перетаскивания ширина меняется на каждое движение, в хранилище пишем по окончании
    const preview = useCallback((raw: number) => setWidth(normalizeWidth(raw)), [])

    const commit = useCallback((raw: number) => {
        const next = normalizeWidth(raw)
        setWidth(next)
        saveWidth(next)
    }, [])

    return { width, compact: isCompact(width), preview, commit }
}
