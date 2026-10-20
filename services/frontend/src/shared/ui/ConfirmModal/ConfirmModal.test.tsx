import { fireEvent, render, screen } from '@testing-library/react'

import { ConfirmModal } from './index'

describe('ConfirmModal', () => {
    it('confirms and cancels', () => {
        const onConfirm = vi.fn()
        const onClose = vi.fn()
        render(
            <ConfirmModal open onClose={onClose} onConfirm={onConfirm} danger>
                Пользователь не сможет вам писать
            </ConfirmModal>,
        )
        expect(screen.getByRole('dialog', { name: 'Вы уверены?' })).toBeInTheDocument()
        expect(screen.getByText('Пользователь не сможет вам писать')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Да' }))
        fireEvent.click(screen.getByRole('button', { name: 'Отмена' }))
        expect(onConfirm).toHaveBeenCalledTimes(1)
        expect(onClose).toHaveBeenCalledTimes(1)
    })
})
