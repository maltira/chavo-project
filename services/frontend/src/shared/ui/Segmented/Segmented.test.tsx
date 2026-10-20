import { fireEvent, render, screen } from '@testing-library/react'

import { Segmented } from './index'

describe('Segmented', () => {
    it('marks the chosen option and reports a new one', () => {
        const onChange = vi.fn()
        render(
            <Segmented
                aria-label="Тип чата"
                value="public"
                onChange={onChange}
                options={[
                    { value: 'public', label: 'Открытый' },
                    { value: 'private', label: 'Закрытый' },
                ]}
            />,
        )
        expect(screen.getByRole('radio', { name: 'Открытый' })).toBeChecked()
        fireEvent.click(screen.getByRole('radio', { name: 'Закрытый' }))
        expect(onChange).toHaveBeenCalledWith('private')
    })
})
