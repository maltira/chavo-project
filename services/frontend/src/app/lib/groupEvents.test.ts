import { QueryClient } from '@tanstack/react-query'

import { useGroupCard } from '@/entities/conversation'

import { applyGroupEvent } from './groupEvents'

describe('applyGroupEvent', () => {
    beforeEach(() => useGroupCard.setState({ target: null }))

    it('rereads the group after a change in it', () => {
        const client = new QueryClient()
        const invalidate = vi.spyOn(client, 'invalidateQueries')
        applyGroupEvent(
            { type: 'conversation.join_request.created', data: { conversation_id: 'g1' } },
            client,
            'me',
        )
        const keys = invalidate.mock.calls.map(([filters]) => filters?.queryKey)
        expect(keys).toEqual(
            expect.arrayContaining([
                ['conversation', 'g1'],
                ['group-members', 'g1'],
                ['join-requests', 'g1'],
                ['bans', 'g1'],
            ]),
        )
    })

    it('closes the window of a group I was removed from', () => {
        const client = new QueryClient()
        client.setQueryData(['conversation', 'g1'], { id: 'g1' })
        useGroupCard.getState().openChat('g1')
        applyGroupEvent(
            {
                type: 'conversation.member.removed',
                data: { conversation_id: 'g1', user_id: 'me' },
            },
            client,
            'me',
        )
        expect(useGroupCard.getState().target).toBeNull()
        expect(client.getQueryData(['conversation', 'g1'])).toBeUndefined()
    })

    it('keeps the window when someone else leaves', () => {
        const client = new QueryClient()
        useGroupCard.getState().openChat('g1')
        applyGroupEvent(
            {
                type: 'conversation.member.removed',
                data: { conversation_id: 'g1', user_id: 'u2' },
            },
            client,
            'me',
        )
        expect(useGroupCard.getState().target).toEqual({ kind: 'chat', id: 'g1' })
    })
})
