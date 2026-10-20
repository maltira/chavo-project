import { fireEvent, render, screen } from '@testing-library/react'

import { Avatar } from './index'

describe('Avatar', () => {
    it('shows the photo when there is one', () => {
        render(<Avatar src="https://example.com/me.png" alt="Фото" size={40} />)
        const image = screen.getByAltText('Фото')
        expect(image).toHaveAttribute('src', 'https://example.com/me.png')
        expect(image).toHaveAttribute('width', '40')
    })

    it('falls back to the placeholder', () => {
        render(<Avatar alt="Фото" />)
        // vite встраивает маленький svg как data-адрес или отдаёт файл: в обоих случаях это svg
        expect(screen.getByAltText('Фото').getAttribute('src')).toMatch(/svg/)
    })

    it('shows the placeholder instead of a broken photo', () => {
        const { rerender } = render(<Avatar src="/media/avatars/gone.jpg" alt="Фото" />)
        const image = screen.getByAltText('Фото')
        expect(image).toHaveAttribute('loading', 'lazy')
        fireEvent.error(image)
        expect(image.getAttribute('src')).toMatch(/svg/)
        // новая ссылка - снова пробуем загрузить
        rerender(<Avatar src="/media/avatars/new.jpg" alt="Фото" />)
        expect(image).toHaveAttribute('src', '/media/avatars/new.jpg')
    })
})
