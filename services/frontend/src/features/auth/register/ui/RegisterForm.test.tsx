import { fireEvent, screen, waitFor } from '@testing-library/react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { register } from '../api/register'
import { RegisterForm } from './RegisterForm'

vi.mock('../api/register', () => ({ register: vi.fn() }))

const registerMock = vi.mocked(register)

function fill(email: string, password: string, confirm: string) {
    fireEvent.change(screen.getByLabelText('Электронная почта'), { target: { value: email } })
    fireEvent.change(screen.getByLabelText('Придумайте пароль'), { target: { value: password } })
    fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: confirm } })
}

const submit = () => fireEvent.click(screen.getByRole('button', { name: 'Присоединиться' }))

describe('RegisterForm', () => {
    beforeEach(() => {
        registerMock.mockReset()
        useNotifications.setState({ items: [] })
    })

    it('shows the password requirements as a hint from the start', () => {
        renderWithProviders(<RegisterForm onSuccess={() => {}} />)
        expect(screen.getByLabelText('Придумайте пароль')).toHaveAccessibleDescription(
            'Минимум 8 символов, заглавная и строчная буква, спецсимвол',
        )
    })

    it('is disabled until every field is filled', () => {
        renderWithProviders(<RegisterForm onSuccess={() => {}} />)
        const button = screen.getByRole('button', { name: 'Присоединиться' })
        expect(button).toBeDisabled()
        fill('user@example.com', 'Secret#1', 'Secret#1')
        expect(button).toBeEnabled()
    })

    it('replaces the hint with the missing requirement after submit', async () => {
        renderWithProviders(<RegisterForm onSuccess={() => {}} />)
        fill('user@example.com', 'secret#1', 'secret#1')
        submit()

        expect(await screen.findByText('Добавьте заглавную букву')).toBeInTheDocument()
        expect(screen.getByLabelText('Придумайте пароль')).toHaveAttribute('aria-invalid', 'true')
        expect(registerMock).not.toHaveBeenCalled()
    })

    it('checks that the passwords match', async () => {
        renderWithProviders(<RegisterForm onSuccess={() => {}} />)
        fill('user@example.com', 'Secret#1', 'Secret#2')
        submit()

        expect(await screen.findByText('Пароли не совпадают')).toBeInTheDocument()
        expect(registerMock).not.toHaveBeenCalled()
    })

    it('sends only email and password and reports the email', async () => {
        registerMock.mockResolvedValue({ message: 'ok' })
        const onSuccess = vi.fn()
        renderWithProviders(<RegisterForm onSuccess={onSuccess} />)
        fill('user@example.com', 'Secret#1', 'Secret#1')
        submit()

        await waitFor(() => expect(onSuccess).toHaveBeenCalledWith('user@example.com'))
        expect(registerMock.mock.calls[0]?.[0]).toEqual({
            email: 'user@example.com',
            password: 'Secret#1',
        })
    })

    it('notifies when the email is already registered', async () => {
        registerMock.mockRejectedValue(new ApiError(409, 'x', 'EMAIL_EXISTS'))
        renderWithProviders(<RegisterForm onSuccess={() => {}} />)
        fill('user@example.com', 'Secret#1', 'Secret#1')
        submit()

        await waitFor(() =>
            expect(useNotifications.getState().items[0]?.description).toBe(
                'Эта почта уже зарегистрирована',
            ),
        )
    })
})
