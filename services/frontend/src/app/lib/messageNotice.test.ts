import { QueryClient } from '@tanstack/react-query'

import { useNotifications } from '@/shared/lib'

import { notifyNewMessage } from './messageNotice'

function setup(conversation_type: 'direct' | 'group') {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } })
    client.setQueryData(['conversations'], {
        pages: [
            [
                {
                    id: 'c1',
                    conversation_type,
                    name: conversation_type === 'group' ? 'Команда' : null,
                },
            ],
        ],
        pageParams: [0],
    })
    client.setQueryData(['user', 'u1'], { user_id: 'u1', display_name: 'Давыд' })
    return client
}

const created = (sender = 'u1') => ({
    type: 'message.created',
    data: { conversation_id: 'c1', sender_id: sender, content: 'Привет!' },
})

const notices = () => useNotifications.getState().items

describe('notifyNewMessage', () => {
    beforeEach(() => {
        useNotifications.setState({ items: [] })
        window.history.replaceState(null, '', '/')
    })

    it('shows the sender and the text for a direct chat', async () => {
        await notifyNewMessage(created(), setup('direct'), 'me')
        expect(notices()).toEqual([
            expect.objectContaining({
                title: 'Давыд',
                description: 'Привет!',
                icon: 'shape-message',
            }),
        ])
    })

    it('shows the group name and who wrote in a group', async () => {
        await notifyNewMessage(created(), setup('group'), 'me')
        expect(notices()[0]).toMatchObject({ title: 'Команда', description: 'Давыд: Привет!' })
    })

    it('opens the chat when the notice is clicked', async () => {
        const open = { chat: vi.fn() }
        await notifyNewMessage(created(), setup('direct'), 'me', open)
        notices()[0]!.onClick!()
        expect(open.chat).toHaveBeenCalledWith('c1')
    })

    it('stays silent for my own messages and for the chat that is open', async () => {
        const client = setup('direct')
        await notifyNewMessage(created('me'), client, 'me')
        window.history.replaceState(null, '', '/chats/c1')
        await notifyNewMessage(created(), client, 'me')
        expect(notices()).toEqual([])
    })

    it('stays silent for a muted chat, including one in the archive', async () => {
        const client = setup('direct')
        client.setQueryData(['conversations'], { pages: [[]], pageParams: [0] })
        client.setQueryData(['conversations', 'archived'], {
            pages: [
                [{ id: 'c1', conversation_type: 'direct', muted_until: '9999-12-31T00:00:00Z' }],
            ],
            pageParams: [0],
        })
        await notifyNewMessage(created(), client, 'me')
        expect(notices()).toEqual([])
    })

    it('stays silent when message has is_silent=true', async () => {
        await notifyNewMessage(
            {
                type: 'message.created',
                data: {
                    conversation_id: 'c1',
                    sender_id: 'u1',
                    content: 'Тихо!',
                    is_silent: true,
                },
            },
            setup('direct'),
            'me',
        )
        expect(notices()).toEqual([])
    })

    it('notifies again once the mute has expired', async () => {
        const client = setup('direct')
        client.setQueryData(['conversations'], {
            pages: [
                [{ id: 'c1', conversation_type: 'direct', muted_until: '2020-01-01T00:00:00Z' }],
            ],
            pageParams: [0],
        })
        await notifyNewMessage(created(), client, 'me')
        expect(notices()).toHaveLength(1)
    })

    it('shows a desktop notification when the tab is hidden', async () => {
        const shown: { title: string; body?: string }[] = []
        class FakeNotification {
            static permission: NotificationPermission = 'granted'
            constructor(title: string, options?: NotificationOptions) {
                shown.push({ title, body: options?.body })
            }
        }
        vi.stubGlobal('Notification', FakeNotification)
        const prev = Object.getOwnPropertyDescriptor(document, 'visibilityState')
        Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
        try {
            await notifyNewMessage(created(), setup('direct'), 'me')
            expect(shown).toEqual([{ title: 'Давыд', body: 'Привет!' }])
        } finally {
            if (prev) {
                Object.defineProperty(document, 'visibilityState', prev)
            } else {
                Object.defineProperty(document, 'visibilityState', {
                    value: 'visible',
                    configurable: true,
                })
            }
            vi.unstubAllGlobals()
        }
    })
})
