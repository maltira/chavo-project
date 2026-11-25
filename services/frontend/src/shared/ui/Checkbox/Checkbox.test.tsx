import { fireEvent, render, screen } from '@testing-library/react'

import { Checkbox } from './Checkbox'

describe('Checkbox', () => {
    it('toggles by clicking the label', () => {
        const onChange = vi.fn()
        render(<Checkbox onChange={onChange}>Также закрепить</Checkbox>)
        const box = screen.getByRole('checkbox', { name: 'Также закрепить' })
        expect(box).not.toBeChecked()
        fireEvent.click(screen.getByText('Также закрепить'))
        expect(box).toBeChecked()
        expect(onChange).toHaveBeenCalledTimes(1)
    })

    it('is disabled', () => {
        render(<Checkbox disabled>Нельзя</Checkbox>)
        expect(screen.getByRole('checkbox', { name: 'Нельзя' })).toBeDisabled()
    })
})
