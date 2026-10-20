import { fireEvent, render, screen } from '@testing-library/react'

import { LanguageMenu } from './index'

const options = [
    { value: 'ru', tag: 'RU', label: 'Русский' },
    { value: 'en', tag: 'EN', label: 'Английский' },
]

function open() {
    fireEvent.click(screen.getByRole('button', { name: 'Выбрать язык' }))
}

describe('LanguageMenu', () => {
    it('is closed until the button is pressed', () => {
        render(<LanguageMenu value="ru" options={options} onChange={() => {}} />)
        const button = screen.getByRole('button', { name: 'Выбрать язык' })
        expect(button).toHaveAttribute('aria-expanded', 'false')
        expect(screen.queryByRole('menu')).toBeNull()

        open()
        expect(button).toHaveAttribute('aria-expanded', 'true')
        expect(screen.getByRole('menuitemradio', { name: 'Русский' })).toHaveAttribute(
            'aria-checked',
            'true',
        )
    })

    it('reports the choice and closes', () => {
        const onChange = vi.fn()
        render(<LanguageMenu value="ru" options={options} onChange={onChange} />)
        open()
        fireEvent.click(screen.getByRole('menuitemradio', { name: 'Английский' }))
        expect(onChange).toHaveBeenCalledWith('en')
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('closes on Escape and on a click outside', () => {
        render(
            <div>
                <p>снаружи</p>
                <LanguageMenu value="ru" options={options} onChange={() => {}} />
            </div>,
        )
        open()
        fireEvent.keyDown(document, { key: 'Escape' })
        expect(screen.queryByRole('menu')).toBeNull()

        open()
        fireEvent.mouseDown(screen.getByText('снаружи'))
        expect(screen.queryByRole('menu')).toBeNull()
    })
})
