import { fireEvent, screen } from '@testing-library/react'

import { useSession } from '@/entities/session'
import { renderWithProviders } from '@/shared/lib/testing'

import { ChangePasswordForm } from './ChangePasswordForm'

const fetchMock = vi.fn()

function fill(current: string, password: string, confirm = password) {
    fireEvent.change(screen.getByLabelText('Текущий пароль'), { target: { value: current } })
    fireEvent.change(screen.getByLabelText('Новый пароль'), { target: { value: password } })
    fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: confirm } })
    fireEvent.click(screen.getByRole('button', { name: 'Изменить пароль' }))
}

describe('ChangePasswordForm', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
        useSession.getState().signIn('tok')
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('changes the password and signs out: every session is over', async () => {
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        renderWithProviders(<ChangePasswordForm />)
        fill('Old-pass1', 'New-pass1!')
        await vi.waitFor(() => expect(useSession.getState().status).toBe('guest'))
        expect(JSON.parse(fetchMock.mock.calls[0]![1].body as string)).toEqual({
            current_password: 'Old-pass1',
            new_password: 'New-pass1!',
        })
    })

    it('marks a wrong current password and does not send mismatched ones', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ error: 'x', reason: 'WRONG_PASSWORD' }), { status: 401 }),
        )
        renderWithProviders(<ChangePasswordForm />)
        fill('wrong', 'New-pass1!', 'Other-pass1!')
        expect(await screen.findByText('Пароли не совпадают')).toBeInTheDocument()
        expect(fetchMock).not.toHaveBeenCalled()

        fireEvent.change(screen.getByLabelText('Повторите пароль'), {
            target: { value: 'New-pass1!' },
        })
        fireEvent.click(screen.getByRole('button', { name: 'Изменить пароль' }))
        expect(await screen.findByText('Неверный текущий пароль')).toBeInTheDocument()
        expect(useSession.getState().status).toBe('authenticated')
    })
})
