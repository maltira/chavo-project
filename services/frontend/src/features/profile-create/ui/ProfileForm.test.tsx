import { fireEvent, screen, waitFor } from '@testing-library/react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { createProfile } from '../api/createProfile'
import { ProfileForm } from './ProfileForm'

vi.mock('../api/createProfile', () => ({ createProfile: vi.fn() }))

const createMock = vi.mocked(createProfile)

function fill(name: string, username: string, bio = '') {
    fireEvent.change(screen.getByLabelText('Ваше имя'), { target: { value: name } })
    fireEvent.change(screen.getByLabelText('Имя пользователя'), { target: { value: username } })
    fireEvent.change(screen.getByLabelText('О себе'), { target: { value: bio } })
    fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
}

describe('ProfileForm', () => {
    beforeEach(() => {
        createMock.mockReset()
        useNotifications.setState({ items: [] })
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

    it('keeps the photo button disabled while uploads are not supported', () => {
        renderWithProviders(<ProfileForm />)
        expect(screen.getByRole('button', { name: 'Выбрать фото' })).toBeDisabled()
    })

    it('draws the @ icon in the username field', () => {
        renderWithProviders(<ProfileForm />)
        const wrapper = screen.getByLabelText('Имя пользователя').parentElement
        expect(wrapper?.querySelector('svg')).not.toBeNull()
    })
})
