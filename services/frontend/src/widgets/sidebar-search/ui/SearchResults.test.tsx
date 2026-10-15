import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import { useGroupCard } from '@/entities/conversation'
import { useProfileCard, useRecentUsers } from '@/entities/user'

import { SearchResults } from './SearchResults'

const profile = (id: string, name: string, username: string) => ({
    user_id: id,
    display_name: name,
    username,
    online: false,
    avatar_shape: null,
})

function renderSearch(query: string) {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], profile('me', 'Я', 'enemybye'))
    client.setQueryData(
        ['users-search', 'arren'],
        [profile('me', 'Я', 'arrenme'), profile('u1', 'Арефий Некрасов', 'arrenek')],
    )
    client.setQueryData(
        ['groups-search', 'arren'],
        [{ id: 'g1', name: 'Arrenoas fans', members_count: 3, is_member: false }],
    )
    client.setQueryData(['user', 'u1'], profile('u1', 'Арефий Некрасов', 'arrenek'))
    client.setQueryData(['conversations'], {
        pages: [[{ id: 'c1', conversation_type: 'direct', peer_id: 'u1', name: null }]],
        pageParams: [0],
    })
    const close = vi.fn()
    render(
        <QueryClientProvider client={client}>
            <MemoryRouter>
                <SearchResults query={query} close={close} />
            </MemoryRouter>
        </QueryClientProvider>,
    )
    return { close }
}

describe('SearchResults', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
        useRecentUsers.setState({ ids: [] })
        useProfileCard.setState({ userId: null })
        useGroupCard.setState({ group: null })
    })
    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    it('shows people without me, groups and my chats after a pause', () => {
        renderSearch('@arren')
        act(() => vi.advanceTimersByTime(300))

        expect(screen.getByText('Общий поиск: 2')).toBeInTheDocument()
        expect(screen.queryByText('@arrenme')).toBeNull()
        expect(screen.getByText('Arrenoas fans')).toBeInTheDocument()
        expect(screen.getByText('Мои чаты: 1')).toBeInTheDocument()
    })

    it('opens the profile from people and the group card from groups', () => {
        renderSearch('arren')
        act(() => vi.advanceTimersByTime(300))

        fireEvent.click(screen.getAllByRole('button', { name: /Арефий Некрасов/ })[0]!)
        expect(useProfileCard.getState().userId).toBe('u1')
        expect(useRecentUsers.getState().ids).toEqual(['u1'])

        fireEvent.click(screen.getByRole('button', { name: /Arrenoas fans/ }))
        expect(useGroupCard.getState().group?.id).toBe('g1')
    })

    it('shows recent profiles and clears them', () => {
        useRecentUsers.setState({ ids: ['u1'] })
        renderSearch('')
        expect(screen.getByRole('button', { name: 'Арефий Некрасов' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Очистить' }))
        expect(screen.queryByText('Недавние')).toBeNull()
    })

    it('hides empty sections', () => {
        renderSearch('nobody')
        act(() => vi.advanceTimersByTime(300))
        expect(screen.queryByText(/Общий поиск/)).toBeNull()
        expect(screen.queryByText(/Мои чаты/)).toBeNull()
        expect(screen.queryByText('Недавние')).toBeNull()
    })
})
