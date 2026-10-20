import { fireEvent, render, screen } from '@testing-library/react'

import { PasswordInput } from './index'

describe('PasswordInput', () => {
    it('hides the password and shows it on demand', () => {
        render(<PasswordInput aria-label="Пароль" />)
        const input = screen.getByLabelText('Пароль')
        expect(input).toHaveAttribute('type', 'password')

        fireEvent.click(screen.getByRole('button', { name: 'Показать пароль' }))
        expect(input).toHaveAttribute('type', 'text')

        fireEvent.click(screen.getByRole('button', { name: 'Скрыть пароль' }))
        expect(input).toHaveAttribute('type', 'password')
    })
})
