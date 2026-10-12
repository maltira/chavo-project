import { fireEvent, render, screen } from '@testing-library/react'

import { Switch } from './index'

describe('Switch', () => {
    it('is a labelled switch, off by default', () => {
        render(<Switch aria-label="Показывать онлайн" />)
        const control = screen.getByRole('switch', { name: 'Показывать онлайн' })
        expect(control).not.toBeChecked()
    })

    it('toggles on click and reports the change', () => {
        const onChange = vi.fn()
        render(<Switch aria-label="Тема" onChange={onChange} />)
        const control = screen.getByRole('switch', { name: 'Тема' })

        fireEvent.click(control)
        expect(control).toBeChecked()
        expect(onChange).toHaveBeenCalledTimes(1)
    })

    it('works as a controlled component', () => {
        const { rerender } = render(
            <Switch aria-label="Тема" checked={false} onChange={() => {}} />,
        )
        expect(screen.getByRole('switch')).not.toBeChecked()

        rerender(<Switch aria-label="Тема" checked onChange={() => {}} />)
        expect(screen.getByRole('switch')).toBeChecked()
    })

    it('can be disabled', () => {
        render(<Switch aria-label="Тема" disabled />)
        expect(screen.getByRole('switch')).toBeDisabled()
    })
})
