import { formatChatTime, formatDay, formatLastSeen } from './date'

describe('formatChatTime', () => {
    const now = new Date(2026, 9, 14, 18, 30) // среда, 14 октября 2026

    it('shows the time for today', () => {
        expect(formatChatTime(new Date(2026, 9, 14, 9, 5).toISOString(), now)).toBe('9:05')
    })

    it('shows the weekday within the last week', () => {
        expect(formatChatTime(new Date(2026, 9, 13, 23, 0).toISOString(), now)).toBe('вт')
        expect(formatChatTime(new Date(2026, 9, 8, 12, 0).toISOString(), now)).toBe('чт')
    })

    it('shows the date for older messages', () => {
        expect(formatChatTime(new Date(2026, 9, 7, 12, 0).toISOString(), now)).toBe('07.10')
        expect(formatChatTime(new Date(2025, 0, 2, 12, 0).toISOString(), now)).toBe('02.01.25')
    })

    it('returns an empty string for a broken date', () => {
        expect(formatChatTime('nope', now)).toBe('')
    })
})

describe('formatDay', () => {
    const now = new Date(2026, 9, 14, 18, 30)

    it('names today, yesterday and older days', () => {
        expect(formatDay(new Date(2026, 9, 14, 1).toISOString(), now)).toBe('Сегодня')
        expect(formatDay(new Date(2026, 9, 13, 23).toISOString(), now)).toBe('Вчера')
        expect(formatDay(new Date(2026, 9, 2).toISOString(), now)).toBe('2 октября')
        expect(formatDay(new Date(2025, 0, 2).toISOString(), now)).toBe('2 января 2025')
    })
})

describe('formatLastSeen', () => {
    const now = new Date(2026, 9, 14, 18, 30)

    it('describes how long ago the user was online', () => {
        expect(formatLastSeen(null, now)).toBe('был(а) недавно')
        expect(formatLastSeen(new Date(2026, 9, 14, 18, 29, 40).toISOString(), now)).toBe(
            'был(а) только что',
        )
        expect(formatLastSeen(new Date(2026, 9, 14, 18, 25).toISOString(), now)).toBe(
            'был(а) 5 мин. назад',
        )
        expect(formatLastSeen(new Date(2026, 9, 14, 15, 0).toISOString(), now)).toBe(
            'был(а) 3 ч. назад',
        )
        expect(formatLastSeen(new Date(2026, 9, 13, 9, 7).toISOString(), now)).toBe(
            'был(а) вчера в 9:07',
        )
        expect(formatLastSeen(new Date(2026, 9, 1, 9, 7).toISOString(), now)).toBe(
            'был(а) 1 октября в 9:07',
        )
    })
})
