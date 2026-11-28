import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'

import type { ConversationSummary } from '@/entities/conversation'
import type { Customization } from '@/entities/customization'
import type { Profile } from '@/entities/user'

import { AvatarShapeEditor } from './AvatarShapeEditor'

const catalog: Customization = {
    status: { icons: ['moon'], durations: ['1h'] },
    avatar_shapes: ['circle', 'bun'],
    profile_cover_patterns: ['star', 'moon'],
    mute_durations: ['1h'],
}

const me: Profile = {
    user_id: 'me',
    username: 'enemybye',
    display_name: 'Андрей',
    online: true,
    last_seen_at: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    avatar_shape: null,
    profile_cover: null,
    status: null,
}

const group: ConversationSummary = {
    id: 'g1',
    conversation_type: 'group',
    visibility: 'public',
    name: 'Клуб',
    description: null,
    avatar_url: null,
    last_message_at: '2026-01-01T00:00:00Z',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    status: null,
    avatar_shape: null,
    profile_cover: { color: '#4F46E5', pattern: 'star' },
    slug: 'club',
    my_role: 'admin',
    peer_id: null,
    last_message: null,
    unread_count: 0,
    last_message_read: false,
    members_count: 3,
    is_saved: false,
    muted_until: null,
    archived_at: null,
    pinned_at: null,
}

const fetchMock = vi.fn()

function setup(opts?: { me?: Profile; group?: ConversationSummary; target?: { kind: 'group'; id: string } }) {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['customization'], catalog)
    client.setQueryData(['me'], opts?.me ?? me)
    client.setQueryData(['conversation', 'g1'], opts?.group ?? group)
    const onDone = vi.fn()
    const view = render(
        <QueryClientProvider client={client}>
            <AvatarShapeEditor target={opts?.target} onDone={onDone} />
        </QueryClientProvider>,
    )
    return { client, onDone, ...view }
}

describe('AvatarShapeEditor (Оформление)', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('switches to Cover tab, picks pattern and saves only cover without touching shape', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ color: '#4F46E5', pattern: 'star' }), { status: 201 }),
        )
        const { client, onDone } = setup()

        fireEvent.click(screen.getByRole('radio', { name: 'Обложка' }))
        fireEvent.click(screen.getByRole('radio', { name: 'Звёзды' }))
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

        await waitFor(() => expect(onDone).toHaveBeenCalled())
        expect(fetchMock).toHaveBeenCalledTimes(1)
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me/profile-cover')
        expect(init.method).toBe('POST')
        expect(JSON.parse(init.body as string)).toEqual({ color: '#4F46E5', pattern: 'star' })
        expect(client.getQueryData<Profile>(['me'])?.profile_cover).toEqual({
            color: '#4F46E5',
            pattern: 'star',
        })
    })

    it('supports gradient-only cover (empty pattern)', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ color: '#4F46E5', pattern: '' }), { status: 201 }),
        )
        const { onDone } = setup()

        fireEvent.click(screen.getByRole('radio', { name: 'Обложка' }))
        fireEvent.click(screen.getByRole('radio', { name: 'Без паттерна' }))
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

        await waitFor(() => expect(onDone).toHaveBeenCalled())
        const [, init] = fetchMock.mock.calls[0]!
        expect(JSON.parse(init.body as string)).toEqual({ color: '#4F46E5', pattern: '' })
    })

    it('removes group profile cover via DELETE endpoint', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        const { client, onDone } = setup({ target: { kind: 'group', id: 'g1' } })

        fireEvent.click(screen.getByRole('radio', { name: 'Обложка' }))
        fireEvent.click(screen.getByRole('button', { name: 'Убрать обложку' }))

        await waitFor(() => expect(onDone).toHaveBeenCalled())
        expect(fetchMock).toHaveBeenCalledWith(
            '/api/conversations/g1/profile-cover',
            expect.objectContaining({ method: 'DELETE' }),
        )
        expect(client.getQueryData<ConversationSummary>(['conversation', 'g1'])?.profile_cover).toBeNull()
    })
})
