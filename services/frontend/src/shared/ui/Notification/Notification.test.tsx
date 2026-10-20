import { fireEvent, render, screen } from '@testing-library/react'

import { Notification } from './index'

describe('Notification', () => {
    it('shows the title and the description', () => {
        render(<Notification title="Нет соединения" description="Проверьте интернет" />)
        expect(screen.getByRole('status')).toHaveTextContent('Нет соединения')
        expect(screen.getByText('Проверьте интернет')).toBeInTheDocument()
    })

    it('works without a description', () => {
        render(<Notification title="Готово" />)
        expect(screen.getByRole('status').querySelectorAll('p')).toHaveLength(1)
    })

    it('shows the close button only with a handler', () => {
        const onClose = vi.fn()
        const { rerender } = render(<Notification title="Готово" />)
        expect(screen.queryByRole('button', { name: 'Закрыть' })).toBeNull()

        rerender(<Notification title="Готово" onClose={onClose} />)
        fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
        expect(onClose).toHaveBeenCalledTimes(1)
    })

    it('is a button only when it has an action', () => {
        const { rerender } = render(<Notification title="Привет" description="текст" />)
        expect(screen.queryByRole('button', { name: /Привет/ })).toBeNull()

        const onClick = vi.fn()
        rerender(<Notification title="Привет" description="текст" onClick={onClick} />)
        fireEvent.click(screen.getByRole('button', { name: /Привет/ }))
        expect(onClick).toHaveBeenCalled()
    })

    it('draws an icon', () => {
        render(<Notification title="Готово" icon="check" />)
        expect(screen.getByRole('status').querySelector('svg')).not.toBeNull()
    })
})
