import { fireEvent, render, screen, within } from '@testing-library/react'

import { PhotoViewer } from './PhotoViewer'
import type { ViewerPhoto } from './PhotoViewer'
import { largeAvatarUrl } from './largeUrl'

const path = (c: string) => `/media/avatars/${c.repeat(32)}.jpg`
const large = (c: string) => `/media/avatars/${c.repeat(32)}_l.jpg`
const photo = (c: string): ViewerPhoto => ({ id: `id-${c}`, src: path(c) })
const history = [photo('a'), photo('b'), photo('c')]

describe('largeAvatarUrl', () => {
    it('points to the large version of our avatars only', () => {
        expect(largeAvatarUrl(path('a'))).toBe(large('a'))
        expect(largeAvatarUrl(large('a'))).toBe(large('a'))
        expect(largeAvatarUrl('https://cdn.example/a.jpg')).toBe('https://cdn.example/a.jpg')
        expect(largeAvatarUrl('/media/avatars/x.jpg')).toBe('/media/avatars/x.jpg')
    })
})

describe('PhotoViewer', () => {
    it('shows the large photo and falls back to the small one for old avatars', () => {
        render(<PhotoViewer photos={[photo('a')]} title="Давыд" onClose={vi.fn()} />)
        const image = document.querySelector('img')!
        expect(image).toHaveAttribute('src', large('a'))
        fireEvent.error(image)
        expect(image).toHaveAttribute('src', path('a'))
    })

    it('closes with the cross, Esc and a click on the dimmed background', () => {
        const onClose = vi.fn()
        render(<PhotoViewer photos={[photo('a')]} title="Давыд" onClose={onClose} />)
        const dialog = screen.getByRole('dialog', { name: 'Фото: Давыд' })

        fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
        fireEvent.keyDown(document, { key: 'Escape' })
        // клик по самому фото окно не закрывает
        fireEvent.mouseDown(document.querySelector('img')!)
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
        render(<PhotoViewer photos={[photo('a')]} title="Давыд" onClose={vi.fn()} />)
        fireEvent.click(screen.getByRole('button', { name: 'Скачать' }))
        expect(click).toHaveBeenCalledTimes(1)
        expect(link?.getAttribute('href')).toBe(large('a'))
        expect(link?.download).toBe('Давыд.jpg')
        vi.restoreAllMocks()
    })

    it('has no arrows or counter for a single photo', () => {
        render(<PhotoViewer photos={[photo('a')]} onClose={vi.fn()} />)
        expect(screen.queryByText(/из \d/)).toBeNull()
        expect(screen.queryByRole('button', { name: /фото$/ })).toBeNull()
    })

    it('pages through the history with arrows and the keyboard', () => {
        render(<PhotoViewer photos={history} onClose={vi.fn()} />)
        const shown = () => document.querySelector('img')!.getAttribute('src')
        expect(screen.getByText('1 из 3')).toBeInTheDocument()
        expect(shown()).toBe(large('a'))

        // влево - более ранние фото, вправо - новее; по кругу
        fireEvent.click(screen.getByRole('button', { name: /более раннее/ }))
        expect(screen.getByText('2 из 3')).toBeInTheDocument()
        expect(shown()).toBe(large('b'))
        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        expect(shown()).toBe(large('c'))
        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        expect(screen.getByText('1 из 3')).toBeInTheDocument()
        fireEvent.keyDown(document, { key: 'ArrowRight' })
        expect(shown()).toBe(large('c'))
        fireEvent.click(screen.getByRole('button', { name: /более новое/ }))
        expect(shown()).toBe(large('b'))
    })

    it('offers to make an older photo the main one, but not the main one itself', () => {
        const onSelect = vi.fn()
        render(<PhotoViewer photos={history} onClose={vi.fn()} onSelect={onSelect} />)
        expect(screen.queryByRole('button', { name: 'Сделать главной' })).toBeNull()

        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        fireEvent.click(screen.getByRole('button', { name: 'Сделать главной' }))
        expect(onSelect).toHaveBeenCalledWith(photo('b'))
    })

    it('does not offer to manage the history without the right to', () => {
        render(<PhotoViewer photos={history} onClose={vi.fn()} />)
        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        expect(screen.queryByRole('button', { name: 'Сделать главной' })).toBeNull()
        expect(screen.queryByRole('button', { name: 'Удалить фото' })).toBeNull()
    })

    it('stays on the same photo when the history is rebuilt, and moves on when it is deleted', () => {
        const { rerender } = render(<PhotoViewer photos={history} onClose={vi.fn()} />)
        const shown = () => document.querySelector('img')!.getAttribute('src')
        fireEvent.keyDown(document, { key: 'ArrowLeft' })
        expect(shown()).toBe(large('b'))

        // «сделали главной» другое фото: список перестроился, просмотр остался на фото b
        rerender(<PhotoViewer photos={[photo('c'), photo('b'), photo('a')]} onClose={vi.fn()} />)
        expect(shown()).toBe(large('b'))
        expect(screen.getByText('2 из 3')).toBeInTheDocument()

        // фото b удалено: показывается соседнее на том же месте
        rerender(<PhotoViewer photos={[photo('c'), photo('a')]} onClose={vi.fn()} />)
        expect(shown()).toBe(large('a'))
        expect(screen.getByText('2 из 2')).toBeInTheDocument()
    })

    it('deletes the shown photo after a confirmation', () => {
        const onDelete = vi.fn()
        const onClose = vi.fn()
        render(<PhotoViewer photos={history} onClose={onClose} onDelete={onDelete} />)
        fireEvent.keyDown(document, { key: 'ArrowLeft' })
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
        // удаляется именно показанное фото, а не главное
        expect(onDelete).toHaveBeenCalledWith(photo('b'))
    })
})
