import { act, fireEvent, screen, waitFor } from '@testing-library/react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { createProfile } from '../api/createProfile'
import { ProfileForm } from './ProfileForm'

vi.mock('../api/createProfile', () => ({ createProfile: vi.fn() }))

// загрузка фото и чтение картинки (в jsdom нет canvas) подменяются
const { uploadMyAvatar, loadImage, encodeSquare } = vi.hoisted(() => ({
    uploadMyAvatar: vi.fn(),
    loadImage: vi.fn(),
    encodeSquare: vi.fn(),
}))
vi.mock('@/entities/user', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/entities/user')>()),
    uploadMyAvatar,
}))
vi.mock('@/shared/lib', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/shared/lib')>()),
    loadImage,
    encodeSquare,
}))

const createMock = vi.mocked(createProfile)

// выбрать фото и подтвердить кроп
async function pickPhoto() {
    const input = screen.getByTestId('photo-input')
    const file = new File([new Uint8Array(10)], 'me.jpg', { type: 'image/jpeg' })
    fireEvent.change(input, { target: { files: [file] } })
    const save = await screen.findByRole('button', { name: 'Сохранить' })
    await waitFor(() => expect(save).toBeEnabled())
    await act(async () => fireEvent.click(save))
}

function fill(name: string, username: string, bio = '') {
    fireEvent.change(screen.getByLabelText('Ваше имя'), { target: { value: name } })
    fireEvent.change(screen.getByLabelText('Имя пользователя'), { target: { value: username } })
    fireEvent.change(screen.getByLabelText('О себе'), { target: { value: bio } })
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
}

describe('ProfileForm', () => {
    beforeEach(() => {
        createMock.mockReset()
        uploadMyAvatar.mockReset()
        useNotifications.setState({ items: [] })
        loadImage.mockResolvedValue({ source: {}, width: 300, height: 300 })
        encodeSquare.mockResolvedValue(new Blob(['jpeg'], { type: 'image/jpeg' }))
        URL.createObjectURL = vi.fn(() => 'blob:preview')
        URL.revokeObjectURL = vi.fn()
    })

    it('strips the leading @ and omits an empty bio', async () => {
        createMock.mockResolvedValue(undefined)
        renderWithProviders(<ProfileForm />)
        fill(' Иван ', '@ivan_99')

        await waitFor(() =>
            expect(createMock.mock.calls[0]?.[0]).toEqual({
                display_name: 'Иван',
                username: 'ivan_99',
                bio: undefined,
            }),
        )
    })

    it('validates the username like the server does', async () => {
        renderWithProviders(<ProfileForm />)
        fill('Иван', 'иван')

        expect(await screen.findByText('Только латиница, цифры и _')).toBeInTheDocument()
        expect(createMock).not.toHaveBeenCalled()
    })

    it('notifies when the username is taken', async () => {
        createMock.mockRejectedValue(new ApiError(409, 'x', 'USERNAME_TAKEN'))
        renderWithProviders(<ProfileForm />)
        fill('Иван', 'ivan')

        await waitFor(() =>
            expect(useNotifications.getState().items[0]?.description).toBe(
                'Это имя пользователя уже занято',
            ),
        )
    })

    it('uploads the chosen photo right after the profile is created', async () => {
        const order: string[] = []
        createMock.mockImplementation(async () => {
            order.push('profile')
        })
        uploadMyAvatar.mockImplementation(async () => {
            order.push('photo')
            return { avatar_url: '/media/avatars/x.jpg' }
        })
        renderWithProviders(<ProfileForm />)
        await pickPhoto()
        // превью выбранного фото, а кнопка меняет подпись
        expect(document.querySelector('img[src="blob:preview"]')).not.toBeNull()
        expect(screen.getByRole('button', { name: 'Сменить фото' })).toBeInTheDocument()

        fill('Иван', 'ivan')
        await waitFor(() => expect(order).toEqual(['profile', 'photo']))
        expect(uploadMyAvatar.mock.calls[0]?.[0]).toBeInstanceOf(Blob)
    })

    it('keeps the created profile when the photo upload fails', async () => {
        createMock.mockResolvedValue(undefined)
        uploadMyAvatar.mockRejectedValue(
            new ApiError(415, 'Можно загрузить только фото', 'AVATAR_TYPE_UNSUPPORTED'),
        )
        renderWithProviders(<ProfileForm />)
        await pickPhoto()
        fill('Иван', 'ivan')

        await waitFor(() =>
            expect(useNotifications.getState().items.map((n) => n.title)).toEqual([
                'Фото не загрузилось',
            ]),
        )
    })

    it('creates the profile without a photo when none was chosen', async () => {
        createMock.mockResolvedValue(undefined)
        renderWithProviders(<ProfileForm />)
        expect(screen.getByRole('button', { name: 'Выбрать фото' })).toBeEnabled()
        fill('Иван', 'ivan')
        await waitFor(() => expect(createMock).toHaveBeenCalled())
        expect(uploadMyAvatar).not.toHaveBeenCalled()
    })

    it('draws the @ icon in the username field', () => {
        renderWithProviders(<ProfileForm />)
        const wrapper = screen.getByLabelText('Имя пользователя').parentElement
        expect(wrapper?.querySelector('svg')).not.toBeNull()
    })
})
