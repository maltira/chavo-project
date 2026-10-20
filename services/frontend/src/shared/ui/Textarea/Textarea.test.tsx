import { render, screen } from '@testing-library/react'

import { Textarea } from './index'

describe('Textarea', () => {
    it('passes attributes to the textarea', () => {
        render(<Textarea placeholder="О себе" maxLength={255} />)
        const field = screen.getByPlaceholderText('О себе')
        expect(field.tagName).toBe('TEXTAREA')
        expect(field).toHaveAttribute('maxlength', '255')
    })

    it('marks an error with aria-invalid', () => {
        render(<Textarea aria-label="О себе" aria-invalid />)
        expect(screen.getByLabelText('О себе')).toHaveAttribute('aria-invalid', 'true')
    })

    it('can be disabled', () => {
        render(<Textarea aria-label="О себе" disabled />)
        expect(screen.getByLabelText('О себе')).toBeDisabled()
    })
})
