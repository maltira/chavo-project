const WEEKDAYS = ['вс', 'пн', 'вт', 'ср', 'чт', 'пт', 'сб']

const pad = (n: number) => String(n).padStart(2, '0')

function startOfDay(date: Date) {
    return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
}

// время в списке чатов: сегодня - «10:16», за последнюю неделю - день недели «пн»,
// в этом году - «14.10», раньше - «14.10.25»
export function formatChatTime(iso: string, now = new Date()): string {
    const date = new Date(iso)
    if (Number.isNaN(date.getTime())) {
        return ''
    }
    const days = Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000)
    if (days <= 0) {
        return `${date.getHours()}:${pad(date.getMinutes())}`
    }
    if (days < 7) {
        return WEEKDAYS[date.getDay()]!
    }
    const day = `${pad(date.getDate())}.${pad(date.getMonth() + 1)}`
    return date.getFullYear() === now.getFullYear()
        ? day
        : `${day}.${String(date.getFullYear()).slice(-2)}`
}

const MONTHS = [
    'января',
    'февраля',
    'марта',
    'апреля',
    'мая',
    'июня',
    'июля',
    'августа',
    'сентября',
    'октября',
    'ноября',
    'декабря',
]

const daysBetween = (date: Date, now: Date) =>
    Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000)

// время сообщения: «9:05»
export function formatTime(iso: string): string {
    const date = new Date(iso)
    return Number.isNaN(date.getTime()) ? '' : `${date.getHours()}:${pad(date.getMinutes())}`
}

// разделитель дней в истории: «Сегодня», «Вчера», «14 октября», «2 января 2025»
export function formatDay(iso: string, now = new Date()): string {
    const date = new Date(iso)
    const days = daysBetween(date, now)
    if (days <= 0) {
        return 'Сегодня'
    }
    if (days === 1) {
        return 'Вчера'
    }
    const day = `${date.getDate()} ${MONTHS[date.getMonth()]}`
    return date.getFullYear() === now.getFullYear() ? day : `${day} ${date.getFullYear()}`
}

// ключ дня для группировки сообщений
export function dayKey(iso: string): string {
    const date = new Date(iso)
    return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
}

// «был(а) …» в шапке чата; null - пользователь скрыл время последнего визита
export function formatLastSeen(iso: string | null, now = new Date()): string {
    if (!iso) {
        return 'был(а) недавно'
    }
    const date = new Date(iso)
    const minutes = Math.floor((now.getTime() - date.getTime()) / 60_000)
    if (minutes < 1) {
        return 'был(а) только что'
    }
    if (minutes < 60) {
        return `был(а) ${minutes} мин. назад`
    }
    const days = daysBetween(date, now)
    if (days <= 0) {
        return `был(а) ${Math.floor(minutes / 60)} ч. назад`
    }
    if (days === 1) {
        return `был(а) вчера в ${formatTime(iso)}`
    }
    return `был(а) ${formatDay(iso, now)} в ${formatTime(iso)}`
}

// полная дата: «13 июля 2026 г.»
export function formatDate(iso: string): string {
    const date = new Date(iso)
    if (Number.isNaN(date.getTime())) {
        return ''
    }
    return `${date.getDate()} ${MONTHS[date.getMonth()]} ${date.getFullYear()} г.`
}
