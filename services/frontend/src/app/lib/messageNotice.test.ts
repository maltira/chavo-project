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

    it('stays silent for my own messages and for the chat that is open', async () => {
        const client = setup('direct')
        await notifyNewMessage(created('me'), client, 'me')
        window.history.replaceState(null, '', '/chats/c1')
        await notifyNewMessage(created(), client, 'me')
        expect(notices()).toEqual([])
    })
})
