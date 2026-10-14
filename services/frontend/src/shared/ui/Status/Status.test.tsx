import { render, screen } from '@testing-library/react'

import { Status } from './index'

const base = { icon: 'moon', text: 'Сегодня отдыхаю', background: '#D97757', foreground: '#512315' }

describe('Status', () => {
    it('shows the text with the given colors', () => {
        render(<Status {...base} />)
        expect(screen.getByText('Сегодня отдыхаю').parentElement).toHaveStyle({
            backgroundColor: '#D97757',
            color: '#512315',
        })
    })

    it('stays visible without expiry and before it', () => {
        const { rerender } = render(<Status {...base} expiresAt={null} />)
        expect(screen.getByText('Сегодня отдыхаю')).toBeInTheDocument()
        rerender(<Status {...base} expiresAt={new Date(Date.now() + 60_000).toISOString()} />)
        expect(screen.getByText('Сегодня отдыхаю')).toBeInTheDocument()
    })

    it('is hidden after expiry', () => {
        render(<Status {...base} expiresAt={new Date(Date.now() - 1000).toISOString()} />)
        expect(screen.queryByText('Сегодня отдыхаю')).toBeNull()
    })
})
