import { fireEvent, render, screen } from '@testing-library/react'

import { Modal } from './index'

describe('Modal', () => {
    it('shows the title and content, closes by the cross, Esc and the backdrop', () => {
        const onClose = vi.fn()
        render(
            <Modal open onClose={onClose} title="Участники">
                <p>содержимое</p>
            </Modal>,
        )
        const dialog = screen.getByRole('dialog', { name: 'Участники' })
        expect(dialog).toHaveFocus()
        expect(screen.getByText('содержимое')).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
        fireEvent.keyDown(document, { key: 'Escape' })
        // клик по самому окну не закрывает, по фону - закрывает
        fireEvent.mouseDown(dialog)
        fireEvent.mouseDown(dialog.parentElement!)
        expect(onClose).toHaveBeenCalledTimes(3)
    })

    it('returns focus to the opener and leaves after the closing animation', () => {
        const opener = document.createElement('button')
        document.body.append(opener)
        opener.focus()

        const { rerender } = render(
            <Modal open onClose={vi.fn()} aria-label="Профиль">
                <p>профиль</p>
            </Modal>,
        )
        rerender(
            <Modal open={false} onClose={vi.fn()} aria-label="Профиль">
                <p>профиль</p>
            </Modal>,
        )
        expect(opener).toHaveFocus()
        const backdrop = screen.getByRole('dialog').parentElement!
        expect(backdrop).toHaveAttribute('data-state', 'closed')
        // в jsdom нет AnimationEvent, и React слушает событие под префиксным именем
        fireEvent(backdrop, new Event('webkitAnimationEnd', { bubbles: true }))
        expect(screen.queryByRole('dialog')).toBeNull()
        opener.remove()
    })
})
