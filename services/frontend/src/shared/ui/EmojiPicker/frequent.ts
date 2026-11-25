// «Часто используемые»: счётчики выбора лежат в браузере (localStorage) - у каждого устройства свои
const KEY = 'chavo:emoji-frequent'
// сколько эмодзи показывает секция (две строки сетки)
export const FREQUENT_LIMIT = 16
// запоминаем не больше стольких, чтобы хранилище не росло бесконечно
const KEEP = 64

type Counts = Record<string, number>

function read(): Counts {
    try {
        const parsed: unknown = JSON.parse(localStorage.getItem(KEY) ?? '{}')
        return parsed && typeof parsed === 'object' ? (parsed as Counts) : {}
    } catch {
        return {}
    }
}

// самые частые первыми; при равенстве - в порядке добавления
export function getFrequent(limit = FREQUENT_LIMIT): string[] {
    return Object.entries(read())
        .sort((a, b) => b[1] - a[1])
        .slice(0, limit)
        .map(([emoji]) => emoji)
}

export function recordEmoji(emoji: string) {
    try {
        const counts = read()
        counts[emoji] = (counts[emoji] ?? 0) + 1
        const kept = Object.entries(counts)
            .sort((a, b) => b[1] - a[1])
            .slice(0, KEEP)
        localStorage.setItem(KEY, JSON.stringify(Object.fromEntries(kept)))
    } catch {
        // хранилище недоступно (приватное окно, заблокировано) - панель работает без «часто используемых»
    }
}
