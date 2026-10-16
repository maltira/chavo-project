import { fireEvent, screen, waitFor } from '@testing-library/react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { login } from '../api/login'
import { LoginForm } from './LoginForm'

vi.mock('../api/login', () => ({ login: vi.fn() }))

const loginMock = vi.mocked(login)

function fill(email: string, password: string) {
    fireEvent.change(screen.getByLabelText('Электронная почта'), { target: { value: email } })
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: password } })
}

describe('LoginForm', () => {
    beforeEach(() => {
        loginMock.mockReset()
        useNotifications.setState({ items: [] })
    })

    it('keeps the submit button disabled until both fields are filled', () => {
        renderWithProviders(<LoginForm onSuccess={() => {}} />)
        const button = screen.getByRole('button', { name: 'Войти' })
        expect(button).toBeDisabled()

        fireEvent.change(screen.getByLabelText('Электронная почта'), {
            target: { value: 'user@example.com' },
        })
        expect(button).toBeDisabled()

        fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret123' } })
        expect(button).toBeEnabled()
    })

    it('does not show field errors just because the user left the field', async () => {
        renderWithProviders(<LoginForm onSuccess={() => {}} />)
        const email = screen.getByLabelText('Электронная почта')
        fireEvent.change(email, { target: { value: 'not-an-email' } })
        fireEvent.blur(email)

        await new Promise((resolve) => setTimeout(resolve, 100))
        expect(screen.queryByText('Введите корректную почту')).toBeNull()
        expect(email).not.toHaveAttribute('aria-invalid')
    })

    it('shows the field error after submitting and clears it once fixed', async () => {
        renderWithProviders(<LoginForm onSuccess={() => {}} />)
        const email = screen.getByLabelText('Электронная почта')
        fill('not-an-email', 'secret123')
        fireEvent.click(screen.getByRole('button', { name: 'Войти' }))

        expect(await screen.findByText('Введите корректную почту')).toBeInTheDocument()
        expect(email).toHaveAttribute('aria-invalid', 'true')
        expect(loginMock).not.toHaveBeenCalled()

        fireEvent.change(email, { target: { value: 'user@example.com' } })
        await waitFor(() => expect(screen.queryByText('Введите корректную почту')).toBeNull())
    })

    it('passes the challenge to the next step on success', async () => {
        loginMock.mockResolvedValue({ challenge_id: 'ch-1', message: 'ok' })
        const onSuccess = vi.fn()
        renderWithProviders(<LoginForm onSuccess={onSuccess} />)

        fill('user@example.com', 'secret123')
        const button = screen.getByRole('button', { name: 'Войти' })
        await waitFor(() => expect(button).toBeEnabled())
        fireEvent.click(button)

        await waitFor(() =>
            expect(onSuccess).toHaveBeenCalledWith({
                challengeId: 'ch-1',
                email: 'user@example.com',
            }),
        )
        expect(loginMock.mock.calls[0]?.[0]).toEqual({
            email: 'user@example.com',
            password: 'secret123',
        })
    })

    it('shows the server error as a notification', async () => {
        loginMock.mockRejectedValue(new ApiError(401, 'x', 'INVALID_CREDENTIALS'))
        renderWithProviders(<LoginForm onSuccess={() => {}} />)

        fill('user@example.com', 'wrong-pass')
        const button = screen.getByRole('button', { name: 'Войти' })
        await waitFor(() => expect(button).toBeEnabled())
        fireEvent.click(button)

        await waitFor(() => expect(useNotifications.getState().items).toHaveLength(1))
        const notice = useNotifications.getState().items[0]
        expect(notice?.title).toBe('Ошибка входа')
        expect(notice?.description).toBe('Неверная почта или пароль')
        expect(screen.queryByRole('alert')).toBeNull()
    })

    it('toggles the password visibility', () => {
        renderWithProviders(<LoginForm onSuccess={() => {}} />)
        const password = screen.getByLabelText('Пароль')
        expect(password).toHaveAttribute('type', 'password')

        fireEvent.click(screen.getByRole('button', { name: 'Показать пароль' }))
        expect(password).toHaveAttribute('type', 'text')
        expect(screen.getByRole('button', { name: 'Скрыть пароль' })).toBeInTheDocument()
    })

    it('links to registration and password reset', () => {
        renderWithProviders(<LoginForm onSuccess={() => {}} />)
        expect(screen.getByRole('link', { name: 'Регистрация' })).toHaveAttribute(
            'href',
            '/auth/register',
        )
        expect(screen.getByRole('link', { name: 'Забыли пароль?' })).toHaveAttribute(
            'href',
            '/auth/reset',
        )
    })
})
