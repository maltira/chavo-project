import { render, screen } from '@testing-library/react'

import { Input } from '../Input'
import { Field } from './index'

describe('Field', () => {
    it('links the label to the field', () => {
        render(<Field label="Электронная почта">{(props) => <Input {...props} />}</Field>)
        expect(screen.getByLabelText('Электронная почта')).toBeInTheDocument()
    })

    it('shows a hint linked to the field without marking it invalid', () => {
        render(
            <Field label="Имя пользователя" hint="По нему вас найдут">
                {(props) => <Input {...props} />}
            </Field>,
        )
        const input = screen.getByLabelText('Имя пользователя')
        expect(input).not.toHaveAttribute('aria-invalid')
        expect(input).toHaveAccessibleDescription('По нему вас найдут')
        expect(screen.queryByRole('alert')).toBeNull()
    })

    it('shows an error instead of the hint and marks the field invalid', () => {
        render(
            <Field label="Пароль" hint="Не меньше 8 символов" error="Неверный пароль">
                {(props) => <Input {...props} />}
            </Field>,
        )
        const input = screen.getByLabelText('Пароль')
        expect(input).toHaveAttribute('aria-invalid', 'true')
        expect(input).toHaveAccessibleDescription('Неверный пароль')
        expect(screen.getByRole('alert')).toHaveTextContent('Неверный пароль')
        expect(screen.queryByText('Не меньше 8 символов')).toBeNull()
    })

    it('renders nothing under the field without hint and error', () => {
        const { container } = render(<Field label="Почта">{(props) => <Input {...props} />}</Field>)
        expect(container.querySelector('p')).toBeNull()
    })
})
