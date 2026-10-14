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
