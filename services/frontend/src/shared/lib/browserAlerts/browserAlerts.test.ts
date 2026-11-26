import { showDesktopNotification, updateTabBadge, useAlertPrefs } from './browserAlerts'

describe('browserAlerts', () => {
    afterEach(() => {
        updateTabBadge(0)
        useAlertPrefs.setState({ sound: true, desktop: true })
        vi.unstubAllGlobals()
    })

    it('updates document title and SVG favicon badge according to unread count', () => {
        const link = document.createElement('link')
        link.rel = 'icon'
        link.type = 'image/svg+xml'
        link.href = '/favicon.svg'
        document.head.appendChild(link)

        updateTabBadge(3)
        expect(document.title).toBe('(3) Chavo')
        expect(link.getAttribute('href')).toMatch(/^data:image\/svg\+xml,/)

        updateTabBadge(150)
        expect(document.title).toBe('(99+) Chavo')

        updateTabBadge(0)
        expect(document.title).toBe('Chavo')
        expect(link.getAttribute('href')).toBe('/favicon.svg')

        link.remove()
    })

    it('creates a desktop Notification when permission is granted and focuses on click', () => {
        const instances: { title: string; options?: NotificationOptions; onclick?: () => void; close: ReturnType<typeof vi.fn> }[] = []
        class FakeNotification {
            static permission: NotificationPermission = 'granted'
            onclick?: () => void
            close = vi.fn()
            constructor(public title: string, public options?: NotificationOptions) {
                instances.push(this)
            }
        }
        vi.stubGlobal('Notification', FakeNotification)
        const focusSpy = vi.spyOn(window, 'focus').mockImplementation(() => {})
        const onClick = vi.fn()

        showDesktopNotification({ title: 'Давыд', body: 'Привет!', tag: 'message-c1', onClick })
        expect(instances).toHaveLength(1)
        expect(instances[0]).toMatchObject({
            title: 'Давыд',
            options: expect.objectContaining({ body: 'Привет!', tag: 'message-c1' }),
        })

        instances[0]!.onclick!()
        expect(focusSpy).toHaveBeenCalled()
        expect(onClick).toHaveBeenCalled()
        expect(instances[0]!.close).toHaveBeenCalled()
        focusSpy.mockRestore()
    })

    it('does not create desktop Notification when disabled in prefs', () => {
        const ctor = vi.fn()
        class FakeNotification {
            static permission: NotificationPermission = 'granted'
            constructor(...args: unknown[]) {
                ctor(...args)
            }
        }
        vi.stubGlobal('Notification', FakeNotification)
        useAlertPrefs.getState().setDesktop(false)

        showDesktopNotification({ title: 'Давыд', body: 'Привет!' })
        expect(ctor).not.toHaveBeenCalled()
    })
})
