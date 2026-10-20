import { render, screen } from '@testing-library/react'

import { IconButton } from './index'

describe('IconButton', () => {
    it('keeps the default size from the stylesheet without the size prop', () => {
        render(<IconButton icon="close" aria-label="Закрыть" />)
        expect(screen.getByRole('button', { name: 'Закрыть' }).style.width).toBe('')
    })

    it('uses the given size and keeps other styles', () => {
        render(<IconButton icon="close" aria-label="Закрыть" size={40} style={{ opacity: 0.5 }} />)
        expect(screen.getByRole('button', { name: 'Закрыть' })).toHaveStyle({
            width: '40px',
            height: '40px',
            opacity: '0.5',
        })
    })

    it('is a plain button by default', () => {
        render(<IconButton icon="close" aria-label="Закрыть" />)
        expect(screen.getByRole('button', { name: 'Закрыть' })).toHaveAttribute('type', 'button')
    })
})
