import { fireEvent, render, screen } from '@testing-library/react'

import { Input } from './index'

describe('Input', () => {
    it('is a text field by default and passes type and placeholder through', () => {
        render(<Input placeholder="Почта" />)
        expect(screen.getByPlaceholderText('Почта')).toHaveAttribute('type', 'text')
    })

    it('keeps the given type, for example password', () => {
        render(<Input type="password" placeholder="Пароль" />)
        expect(screen.getByPlaceholderText('Пароль')).toHaveAttribute('type', 'password')
    })

    it('draws the left icon as decoration, not as a button', () => {
        const { container } = render(<Input leftIcon="user" />)
        expect(container.querySelectorAll('svg')).toHaveLength(1)
        expect(screen.queryByRole('button')).toBeNull()
    })

    it('makes the right icon a labelled button when there is a handler', () => {
        const onClick = vi.fn()
        render(
            <Input
                type="password"
                rightIcon="eye-closed"
                rightIconLabel="Показать пароль"
                onRightIconClick={onClick}
            />,
        )
        fireEvent.click(screen.getByRole('button', { name: 'Показать пароль' }))
        expect(onClick).toHaveBeenCalledTimes(1)
    })

    it('does not submit a form when the right icon is clicked', () => {
        const onSubmit = vi.fn((e: { preventDefault: () => void }) => e.preventDefault())
        render(
            <form onSubmit={onSubmit}>
                <Input
                    rightIcon="eye-closed"
                    rightIconLabel="Показать"
                    onRightIconClick={() => {}}
                />
            </form>,
        )
        fireEvent.click(screen.getByRole('button', { name: 'Показать' }))
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('marks an error with aria-invalid on the field', () => {
        render(<Input aria-label="Почта" aria-invalid />)
        expect(screen.getByLabelText('Почта')).toHaveAttribute('aria-invalid', 'true')
    })
})
