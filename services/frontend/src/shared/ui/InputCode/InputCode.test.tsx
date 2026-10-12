import { fireEvent, render, screen } from '@testing-library/react'

import { InputCode } from './index'

describe('InputCode', () => {
    it('is a numeric single field with the "_" placeholder by default', () => {
        render(<InputCode aria-label="Цифра 1 из 6" />)
        const input = screen.getByLabelText('Цифра 1 из 6')
        expect(input).toHaveAttribute('placeholder', '_')
        expect(input).toHaveAttribute('inputmode', 'numeric')
        expect(input).toHaveAttribute('type', 'text')
    })

    it('accepts a custom placeholder', () => {
        render(<InputCode aria-label="Цифра" placeholder="•" />)
        expect(screen.getByLabelText('Цифра')).toHaveAttribute('placeholder', '•')
    })

    it('keeps only a digit', () => {
        const onChange = vi.fn()
        render(<InputCode aria-label="Цифра" onChange={onChange} />)
        const input = screen.getByLabelText('Цифра') as HTMLInputElement

        fireEvent.change(input, { target: { value: 'a' } })
        expect(input.value).toBe('')

        fireEvent.change(input, { target: { value: '7' } })
        expect(input.value).toBe('7')
        expect(onChange).toHaveBeenCalledTimes(2)
    })

    it('replaces the old digit with the new one', () => {
        render(<InputCode aria-label="Цифра" defaultValue="1" />)
        const input = screen.getByLabelText('Цифра') as HTMLInputElement

        fireEvent.change(input, { target: { value: '15' } })
        expect(input.value).toBe('5')
    })

    it('selects the digit on focus', () => {
        render(<InputCode aria-label="Цифра" defaultValue="4" />)
        const input = screen.getByLabelText('Цифра') as HTMLInputElement

        fireEvent.focus(input)
        expect(input.selectionStart).toBe(0)
        expect(input.selectionEnd).toBe(1)
    })
})
