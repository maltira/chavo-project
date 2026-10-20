import { fireEvent, render, screen } from '@testing-library/react'

import { MenuItem } from '../MenuItem'
import { ContextMenu } from './index'

describe('ContextMenu', () => {
    it('opens at the position and closes after choosing an item', () => {
        const onClose = vi.fn()
        const onRead = vi.fn()
        render(
            <ContextMenu position={{ x: 10, y: 20 }} onClose={onClose} aria-label="Действия">
                <MenuItem icon="check-double" onClick={onRead}>
                    Прочитать чат
                </MenuItem>
            </ContextMenu>,
        )
        const menu = screen.getByRole('menu', { name: 'Действия' })
        expect(menu).toHaveStyle({ left: '10px', top: '20px' })
        fireEvent.click(screen.getByRole('menuitem', { name: 'Прочитать чат' }))
        expect(onRead).toHaveBeenCalled()
        expect(onClose).toHaveBeenCalled()
    })

    it('renders nothing without a position', () => {
        render(
            <ContextMenu position={null} onClose={vi.fn()} aria-label="Действия">
                <span>пункт</span>
            </ContextMenu>,
        )
        expect(screen.queryByRole('menu')).toBeNull()
    })
})
