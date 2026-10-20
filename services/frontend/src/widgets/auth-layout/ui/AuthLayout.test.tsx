import { fireEvent, render, screen } from '@testing-library/react'

import { AuthCard, AuthLayout } from '../index'

describe('AuthLayout', () => {
    it('renders the content with the theme and language controls', () => {
        render(
            <AuthLayout>
                <p>форма</p>
            </AuthLayout>,
        )
        expect(screen.getByText('форма')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /тему/ })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Выбрать язык' })).toBeInTheDocument()
    })

    it('links the footer to Telegram with a single accessible link', () => {
        render(<AuthLayout>форма</AuthLayout>)
        const link = screen.getByRole('link', { name: 'Telegram' })
        expect(link).toHaveAttribute('href', 'https://t.me/enemybye')
        expect(link).toHaveAttribute('target', '_blank')
        expect(link).toHaveAttribute('rel', 'noreferrer noopener')
        expect(screen.queryByRole('button', { name: 'Telegram' })).toBeNull()
        expect(screen.getByText('ENEMYBYE')).toBeInTheDocument()
    })

    it('hides the footer and the logo on request', () => {
        const { container } = render(
            <AuthLayout showFooter={false} showLogo={false}>
                форма
            </AuthLayout>,
        )
        expect(screen.queryByRole('link', { name: 'Telegram' })).toBeNull()
        expect(container.querySelectorAll('svg').length).toBeGreaterThan(0)
        expect(screen.queryByText('ENEMYBYE')).toBeNull()
    })
})

describe('AuthCard', () => {
    it('draws the back button with a known icon', () => {
        render(<AuthCard onBack={() => {}}>карточка</AuthCard>)
        const button = screen.getByRole('button', { name: 'Назад' })
        expect(button.querySelector('svg')).not.toBeNull()
    })

    it('shows the back button only with a handler', () => {
        const onBack = vi.fn()
        const { rerender } = render(<AuthCard>карточка</AuthCard>)
        expect(screen.queryByRole('button', { name: 'Назад' })).toBeNull()

        rerender(<AuthCard onBack={onBack}>карточка</AuthCard>)
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(onBack).toHaveBeenCalledTimes(1)
    })
})
