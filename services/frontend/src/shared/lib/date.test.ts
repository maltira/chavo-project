import { formatChatTime } from './date'

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
