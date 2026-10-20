import { fireEvent, screen, waitFor } from '@testing-library/react'

import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { confirmReset, requestReset } from '../api/reset'
import { ResetConfirmForm } from './ResetConfirmForm'
import { ResetRequestForm } from './ResetRequestForm'

vi.mock('../api/reset', () => ({ requestReset: vi.fn(), confirmReset: vi.fn() }))

const requestMock = vi.mocked(requestReset)
const confirmMock = vi.mocked(confirmReset)

beforeEach(() => {
    requestMock.mockReset()
    confirmMock.mockReset()
    useNotifications.setState({ items: [] })
})

describe('ResetRequestForm', () => {
    it('sends the email and passes the server message on', async () => {
        requestMock.mockResolvedValue({ message: 'Ссылка отправлена' })
        const onSuccess = vi.fn()
        renderWithProviders(<ResetRequestForm onSuccess={onSuccess} />)

        fireEvent.change(screen.getByLabelText('Электронная почта'), {
            target: { value: 'user@example.com' },
        })
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))

        await waitFor(() => expect(onSuccess).toHaveBeenCalledWith('Ссылка отправлена'))
        expect(requestMock.mock.calls[0]?.[0]).toEqual({ email: 'user@example.com' })
    })
})

describe('ResetConfirmForm', () => {
    function fill(password: string, confirm: string) {
        fireEvent.change(screen.getByLabelText('Придумайте пароль'), {
            target: { value: password },
        })
        fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: confirm } })
        fireEvent.click(screen.getByRole('button', { name: 'Изменить пароль' }))
    }

    it('sends the token with the new password', async () => {
        confirmMock.mockResolvedValue({ message: 'ok' })
        const onSuccess = vi.fn()
        renderWithProviders(
            <ResetConfirmForm token="tok" onSuccess={onSuccess} onInvalidToken={() => {}} />,
        )
        fill('Secret#1', 'Secret#1')

        await waitFor(() => expect(onSuccess).toHaveBeenCalled())
        expect(confirmMock.mock.calls[0]?.[0]).toEqual({ token: 'tok', password: 'Secret#1' })
    })

    it('applies the password rules', async () => {
        renderWithProviders(
            <ResetConfirmForm token="tok" onSuccess={() => {}} onInvalidToken={() => {}} />,
        )
        fill('Secret12', 'Secret12')
        expect(await screen.findByText('Добавьте спецсимвол, например ! или #')).toBeInTheDocument()
        expect(confirmMock).not.toHaveBeenCalled()
    })

    it('reports an expired link instead of a notification', async () => {
        confirmMock.mockRejectedValue(new ApiError(400, 'x', 'INVALID_TOKEN'))
        const onInvalidToken = vi.fn()
        renderWithProviders(
            <ResetConfirmForm token="tok" onSuccess={() => {}} onInvalidToken={onInvalidToken} />,
        )
        fill('Secret#1', 'Secret#1')

        await waitFor(() => expect(onInvalidToken).toHaveBeenCalled())
        expect(useNotifications.getState().items).toHaveLength(0)
    })
})
