import { fireEvent, render, screen } from '@testing-library/react'

import { ColorPicker } from './index'

describe('ColorPicker', () => {
    it('picks a palette color and any custom one in upper case', () => {
        const onChange = vi.fn()
        render(
            <ColorPicker
                aria-label="Фон"
                value="#d97757"
                palette={['#D97757', '#25E87F']}
                onChange={onChange}
            />,
        )
        expect(screen.getByRole('radio', { name: '#D97757' })).toBeChecked()
        fireEvent.click(screen.getByRole('radio', { name: '#25E87F' }))
        expect(onChange).toHaveBeenLastCalledWith('#25E87F')

        fireEvent.change(screen.getByLabelText('Свой цвет'), { target: { value: '#123abc' } })
        expect(onChange).toHaveBeenLastCalledWith('#123ABC')
    })
})
