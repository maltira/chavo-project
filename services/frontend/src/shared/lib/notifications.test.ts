import { dismiss, notify, useNotifications } from './notifications'

describe('notifications', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        useNotifications.setState({ items: [] })
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('adds a notification and removes it after the timeout', () => {
        notify({ title: 'Готово' })
        expect(useNotifications.getState().items).toHaveLength(1)

        vi.advanceTimersByTime(5000)
        expect(useNotifications.getState().items).toHaveLength(0)
    })

    it('keeps a persistent notification until it is dismissed', () => {
        const id = notify({ title: 'Нет соединения', persistent: true })
        vi.advanceTimersByTime(60_000)
        expect(useNotifications.getState().items).toHaveLength(1)

        dismiss(id)
        expect(useNotifications.getState().items).toHaveLength(0)
    })

    it('does not duplicate a notification with the same id and restarts its timer', () => {
        notify({ id: 'network', title: 'Нет соединения' })
        vi.advanceTimersByTime(4000)
        notify({ id: 'network', title: 'Нет соединения' })

        expect(useNotifications.getState().items).toHaveLength(1)
        vi.advanceTimersByTime(4000)
        expect(useNotifications.getState().items).toHaveLength(1)
        vi.advanceTimersByTime(1000)
        expect(useNotifications.getState().items).toHaveLength(0)
    })

    it('shows at most three notifications, newest last', () => {
        notify({ title: '1' })
        notify({ title: '2' })
        notify({ title: '3' })
        notify({ title: '4' })

        expect(useNotifications.getState().items.map((item) => item.title)).toEqual(['2', '3', '4'])
    })
})
