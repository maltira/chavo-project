import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'

import type { Profile } from '@/entities/user'
import { renderWithProviders } from '@/shared/lib/testing'

import { ProfileEditForm } from './ProfileEditForm'

const profile = {
    user_id: 'me',
    display_name: 'Андрей',
    username: 'enemybye',
    bio: 'Пишу мессенджер',
} as Profile

const fetchMock = vi.fn()

// чтение и кодирование фото в jsdom недоступны (нет canvas): подменяются
const { loadImage, encodeSquare } = vi.hoisted(() => ({
    loadImage: vi.fn(),
    encodeSquare: vi.fn(),
}))
vi.mock('@/shared/lib', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/shared/lib')>()),
    loadImage,
    encodeSquare,
}))

// выбрать фото и подтвердить кроп
async function pickPhoto() {
    loadImage.mockResolvedValue({ source: {}, width: 300, height: 300 })
    encodeSquare.mockResolvedValue(new Blob(['jpeg'], { type: 'image/jpeg' }))
    const file = new File([new Uint8Array(10)], 'me.jpg', { type: 'image/jpeg' })
    fireEvent.change(screen.getByTestId('photo-input'), { target: { files: [file] } })
    const save = await screen.findByRole('button', { name: 'Сохранить' })
    await waitFor(() => expect(save).toBeEnabled())
    await act(async () => fireEvent.click(save))
}

describe('ProfileEditForm', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('sends only the changed fields', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        const onDone = vi.fn()
        renderWithProviders(<ProfileEditForm profile={profile} onDone={onDone} />)

        const save = screen.getByRole('button', { name: 'Сохранить изменения' })
        expect(save).toBeDisabled()
        fireEvent.change(screen.getByLabelText('О себе'), { target: { value: 'Новое о себе' } })
        fireEvent.click(save)

        await vi.waitFor(() => expect(onDone).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me')
        expect(init.method).toBe('PATCH')
        expect(JSON.parse(init.body as string)).toEqual({ bio: 'Новое о себе' })
    })

    it('shows a taken username at the field', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ error: 'x', reason: 'USERNAME_TAKEN' }), { status: 409 }),
        )
        renderWithProviders(<ProfileEditForm profile={profile} onDone={vi.fn()} />)
        fireEvent.change(screen.getByLabelText('Имя пользователя'), { target: { value: 'taken' } })
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }))
        expect(await screen.findByText('Это имя пользователя уже занято')).toBeInTheDocument()
    })

    it('compares with the profile at the moment the form was opened', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        const onDone = vi.fn()
        const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
        const tree = (p: Profile) => (
            <QueryClientProvider client={client}>
                <ProfileEditForm profile={p} onDone={onDone} />
            </QueryClientProvider>
        )
        const { rerender } = render(tree(profile))
        // профиль обновился, пока форма открыта (например, перечитался)
        rerender(tree({ ...profile, bio: 'Уже другое' }))
        fireEvent.change(screen.getByLabelText('О себе'), { target: { value: 'Уже другое' } })
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить изменения' }))

        await vi.waitFor(() => expect(onDone).toHaveBeenCalled())
        expect(JSON.parse(fetchMock.mock.calls[0]![1].body as string)).toEqual({
            bio: 'Уже другое',
        })
    })

    it('uploads a new photo at once and removes it after confirmation', async () => {
        fetchMock.mockImplementation((_url: string, init: RequestInit) =>
            Promise.resolve(
                init.method === 'POST'
                    ? new Response(JSON.stringify({ avatar_url: '/media/avatars/new.jpg' }))
                    : new Response('{"message":"ok"}'),
            ),
        )
        renderWithProviders(
            <ProfileEditForm
                profile={{ ...profile, avatar_url: '/media/avatars/old.jpg' }}
                onDone={vi.fn()}
            />,
        )
        await pickPhoto()
        await waitFor(() => expect(fetchMock).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me/avatar')
        expect(init.method).toBe('POST')
        expect(init.body).toBeInstanceOf(FormData)
        expect((init.body as FormData).get('file')).toBeInstanceOf(Blob)

        fireEvent.click(screen.getByRole('button', { name: 'Удалить фото' }))
        fireEvent.click(screen.getByRole('button', { name: 'Удалить' }))
        await waitFor(() =>
            expect(
                fetchMock.mock.calls.some(
                    ([u, i]) => u === '/api/users/me/avatar' && i.method === 'DELETE',
                ),
            ).toBe(true),
        )
    })
})
