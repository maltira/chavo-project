import { fireEvent, render, screen } from '@testing-library/react'

import { TextLink } from './index'

describe('TextLink', () => {
    it('is a link by default', () => {
        render(<TextLink href="/auth/register">Регистрация</TextLink>)
        const link = screen.getByRole('link', { name: 'Регистрация' })
        expect(link).toHaveAttribute('href', '/auth/register')
    })

    it('can be a button that does not submit a form', () => {
        const onSubmit = vi.fn((event: { preventDefault: () => void }) => event.preventDefault())
        const onClick = vi.fn()
        render(
            <form onSubmit={onSubmit}>
                <TextLink as="button" onClick={onClick}>
                    Отправить снова
                </TextLink>
            </form>,
        )
        const button = screen.getByRole('button', { name: 'Отправить снова' })
        expect(button).toHaveAttribute('type', 'button')

        fireEvent.click(button)
        expect(onClick).toHaveBeenCalledTimes(1)
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('renders through another component and passes its props', () => {
        function Custom({
            to,
            children,
            className,
        }: {
            to: string
            children: React.ReactNode
            className?: string
        }) {
            return (
                <a href={to} className={className} data-custom>
                    {children}
                </a>
            )
        }
        render(
            <TextLink as={Custom} to="/auth/login">
                Войти
            </TextLink>,
        )
        const link = screen.getByRole('link', { name: 'Войти' })
        expect(link).toHaveAttribute('href', '/auth/login')
        expect(link).toHaveAttribute('data-custom')
    })
})
