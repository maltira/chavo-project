import { fireEvent, render, screen } from '@testing-library/react'

import { Switch } from '../Switch'
import { MenuItem } from './index'

describe('MenuItem', () => {
    it('is a button with onClick', () => {
        const onClick = vi.fn()
        render(
            <MenuItem icon="exit" onClick={onClick}>
                Выйти
            </MenuItem>,
        )
        fireEvent.click(screen.getByRole('menuitem', { name: 'Выйти' }))
        expect(onClick).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('menuitem').tagName).toBe('BUTTON')
    })

    it('is a plain row without onClick', () => {
        render(<MenuItem icon="settings">Настройки</MenuItem>)
        expect(screen.getByRole('menuitem').tagName).toBe('DIV')
    })

    it('toggles the trailing control when the row is clicked', () => {
        const onChange = vi.fn()
        render(
            <MenuItem icon="theme-dark" trailing={<Switch aria-label="Тема" onChange={onChange} />}>
                Тёмная тема
            </MenuItem>,
        )
        fireEvent.click(screen.getByText('Тёмная тема'))
        expect(onChange).toHaveBeenCalledTimes(1)
    })

    it('does not fire when disabled', () => {
        const onClick = vi.fn()
        render(
            <MenuItem icon="exit" disabled onClick={onClick}>
                Выйти
            </MenuItem>,
        )
        expect(screen.getByRole('menuitem')).toBeDisabled()
    })
})
