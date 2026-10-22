import { fireEvent, render, screen, within } from '@testing-library/react'

import { PhotoViewer } from './PhotoViewer'
import { largeAvatarUrl } from './largeUrl'

const NAME = 'a'.repeat(32)
const SMALL = `/media/avatars/${NAME}.jpg`
const LARGE = `/media/avatars/${NAME}_l.jpg`

describe('largeAvatarUrl', () => {
    it('points to the large version of our avatars only', () => {
        expect(largeAvatarUrl(SMALL)).toBe(LARGE)
        expect(largeAvatarUrl(LARGE)).toBe(LARGE)
        expect(largeAvatarUrl('https://cdn.example/a.jpg')).toBe('https://cdn.example/a.jpg')
        expect(largeAvatarUrl('/media/avatars/x.jpg')).toBe('/media/avatars/x.jpg')
    })
})

describe('PhotoViewer', () => {
    it('shows the large photo and falls back to the small one for old avatars', () => {
        render(<PhotoViewer src={SMALL} title="Давыд" onClose={vi.fn()} />)
        const photo = screen.getByRole('img', { name: 'Давыд' })
        expect(photo).toHaveAttribute('src', LARGE)
        fireEvent.error(photo)
        expect(photo).toHaveAttribute('src', SMALL)
    })

    it('closes with the cross, Esc and a click on the dimmed background', () => {
        const onClose = vi.fn()
        render(<PhotoViewer src={SMALL} title="Давыд" onClose={onClose} />)
        const dialog = screen.getByRole('dialog', { name: 'Фото: Давыд' })

        fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
        fireEvent.keyDown(document, { key: 'Escape' })
        // клик по самому фото окно не закрывает
        fireEvent.mouseDown(screen.getByRole('img', { name: 'Давыд' }))
        expect(onClose).toHaveBeenCalledTimes(2)
        fireEvent.mouseDown(dialog)
        expect(onClose).toHaveBeenCalledTimes(3)
    })

    it('downloads the shown file under the person name', () => {
        const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
        let link: HTMLAnchorElement | undefined
        const create = document.createElement.bind(document)
        vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
            const el = create(tag)
            if (tag === 'a') {
                link = el as HTMLAnchorElement
            }
            return el
        })
        render(<PhotoViewer src={SMALL} title="Давыд" onClose={vi.fn()} />)
        fireEvent.click(screen.getByRole('button', { name: 'Скачать' }))
        expect(click).toHaveBeenCalledTimes(1)
        expect(link?.getAttribute('href')).toBe(LARGE)
        expect(link?.download).toBe('Давыд.jpg')
        vi.restoreAllMocks()
    })

    it('offers deletion only when allowed, after a confirmation', () => {
        const { rerender } = render(<PhotoViewer src={SMALL} onClose={vi.fn()} />)
        expect(screen.queryByRole('button', { name: 'Удалить фото' })).toBeNull()

        const onDelete = vi.fn()
        const onClose = vi.fn()
        rerender(<PhotoViewer src={SMALL} onClose={onClose} onDelete={onDelete} />)
        fireEvent.click(screen.getByRole('button', { name: 'Удалить фото' }))
        expect(onDelete).not.toHaveBeenCalled()

        // Esc при открытом подтверждении закрывает только его, фото остаётся
        fireEvent.keyDown(document, { key: 'Escape' })
        expect(onClose).not.toHaveBeenCalled()
        // окно подтверждения играет анимацию закрытия: оно ещё в разметке, но уже закрывается
        expect(screen.getByRole('dialog', { name: 'Вы уверены?' }).parentElement).toHaveAttribute(
            'data-state',
            'closed',
        )

        fireEvent.click(screen.getByRole('button', { name: 'Удалить фото' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Удалить' }))
        expect(onDelete).toHaveBeenCalledTimes(1)
    })
})
