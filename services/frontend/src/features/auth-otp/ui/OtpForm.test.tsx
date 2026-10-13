import { act, fireEvent, screen, waitFor } from '@testing-library/react'

import { useSession } from '@/entities/session'
import { ApiError } from '@/shared/api'
import { useNotifications } from '@/shared/lib'
import { renderWithProviders } from '@/shared/lib/testing'

import { resendOtp, verifyOtp } from '../api/otp'
import { OtpForm } from './OtpForm'

vi.mock('../api/otp', () => ({ verifyOtp: vi.fn(), resendOtp: vi.fn() }))

const verifyMock = vi.mocked(verifyOtp)
const resendMock = vi.mocked(resendOtp)

function cell(index: number) {
    return screen.getByLabelText(`Цифра ${index} из 6`) as HTMLInputElement
}

function typeCode(code: string) {
    code.split('').forEach((digit, i) => {
        fireEvent.change(cell(i + 1), { target: { value: digit } })
    })
}

function renderForm(onExpired = vi.fn()) {
    renderWithProviders(
        <OtpForm challengeId="ch-1" email="user@example.com" onExpired={onExpired} />,
    )
    return onExpired
}

describe('OtpForm', () => {
    beforeEach(() => {
        verifyMock.mockReset()
        resendMock.mockReset()
        useNotifications.setState({ items: [] })
        useSession.setState({ status: 'guest' })
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('shows the email and six empty fields', () => {
        renderForm()
        expect(screen.getByText('user@example.com')).toBeInTheDocument()
        for (let i = 1; i <= 6; i++) {
            expect(cell(i).value).toBe('')
        }
        expect(screen.getByRole('button', { name: 'Продолжить' })).toBeDisabled()
    })

    it('moves focus to the next field after a digit', () => {
        renderForm()
        fireEvent.change(cell(1), { target: { value: '4' } })
        expect(cell(2)).toHaveFocus()
    })

    it('goes back on Backspace in an empty field', () => {
        renderForm()
        typeCode('12')
        expect(cell(3)).toHaveFocus()
        fireEvent.keyDown(cell(3), { key: 'Backspace' })
        expect(cell(2)).toHaveFocus()
        expect(cell(2).value).toBe('')
    })

    it('spreads a pasted code over the fields', () => {
        renderForm()
        fireEvent.paste(cell(1), { clipboardData: { getData: () => '48 29-13' } })
        expect(cell(1).value).toBe('4')
        expect(cell(6).value).toBe('3')
        expect(screen.getByRole('button', { name: 'Продолжить' })).toBeEnabled()
    })

    it('signs in with the access token on a correct code', async () => {
        verifyMock.mockResolvedValue({ access_token: 'tok', token_type: 'Bearer' })
        renderForm()
        typeCode('482913')
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))

        await waitFor(() => expect(useSession.getState().status).toBe('authenticated'))
        expect(verifyMock.mock.calls[0]?.[0]).toEqual({ challenge_id: 'ch-1', code: '482913' })
    })

    it('marks the fields invalid and notifies on a wrong code, then clears the mark on edit', async () => {
        verifyMock.mockRejectedValue(new ApiError(400, 'x', 'INVALID_OTP'))
        renderForm()
        typeCode('111111')
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))

        await waitFor(() => expect(cell(1)).toHaveAttribute('aria-invalid', 'true'))
        const notice = useNotifications.getState().items[0]
        expect(notice?.description).toBe('Неверный или недействительный код')
        expect(screen.queryByRole('alert')).toBeNull()

        fireEvent.change(cell(6), { target: { value: '2' } })
        await waitFor(() => expect(cell(1)).not.toHaveAttribute('aria-invalid'))
    })

    it('returns to the first step when the challenge expired', async () => {
        verifyMock.mockRejectedValue(new ApiError(400, 'x', 'INVALID_CHALLENGE'))
        const onExpired = renderForm()
        typeCode('111111')
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))

        await waitFor(() => expect(onExpired).toHaveBeenCalledTimes(1))
    })

    it('goes back to the first step when attempts are exhausted without telling why', async () => {
        verifyMock.mockRejectedValue(new ApiError(429, 'x', 'OTP_ATTEMPTS_EXCEEDED'))
        const onExpired = renderForm()
        typeCode('111111')
        fireEvent.click(screen.getByRole('button', { name: 'Продолжить' }))

        await waitFor(() => expect(onExpired).toHaveBeenCalledTimes(1))
        expect(useNotifications.getState().items).toHaveLength(0)
    })

    it('allows resending only after the cooldown', async () => {
        vi.useFakeTimers()
        resendMock.mockResolvedValue({ message: 'ok' })
        renderForm()
        // кнопка повтора стоит в строке «Код не пришёл?»: на паузе в ней секунды, потом текст
        const resend = () =>
            screen.getByText('Код не пришёл?').parentElement!.querySelector('button')!

        expect(resend()).toBeDisabled()
        expect(resend()).toHaveTextContent('60 с')

        for (let i = 0; i < 60; i++) {
            await act(async () => {
                await vi.advanceTimersByTimeAsync(1000)
            })
        }
        expect(resend()).toBeEnabled()
        expect(resend()).toHaveTextContent('Отправить снова')

        fireEvent.click(resend())
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(resendMock.mock.calls[0]?.[0]).toEqual({ challenge_id: 'ch-1' })
        expect(resend()).toBeDisabled()
    })
})
