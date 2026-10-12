import { act, render, screen } from '@testing-library/react'

import { Loader } from './index'

// число кадров берём из папки: добавили или убрали файл - тест подстроится сам
const FRAME_COUNT = Object.keys(import.meta.glob('./frames/*.svg')).length

function activeFrames(container: HTMLElement) {
    return container.querySelectorAll('svg[data-active="true"]')
}

describe('Loader', () => {
    afterEach(() => {
        vi.useRealTimers()
    })

    it('has frames to animate between', () => {
        expect(FRAME_COUNT).toBeGreaterThan(1)
    })

    it('is a status with every frame stacked and exactly one active', () => {
        const { container } = render(<Loader />)
        expect(screen.getByRole('status')).toBeInTheDocument()
        expect(container.querySelectorAll('svg')).toHaveLength(FRAME_COUNT)
        expect(activeFrames(container)).toHaveLength(1)
    })

    it('hides the frames from assistive tech', () => {
        const { container } = render(<Loader />)
        container.querySelectorAll('svg').forEach((svg) => {
            expect(svg).toHaveAttribute('aria-hidden', 'true')
        })
    })

    it('uses the given size and color', () => {
        render(<Loader size={32} color="#D97757" />)
        expect(screen.getByRole('status')).toHaveStyle({
            width: '32px',
            height: '32px',
            color: '#D97757',
        })
    })

    it('has no accent circle by default', () => {
        render(<Loader />)
        expect(screen.getByRole('status').className).not.toMatch(/withContainer/)
    })

    it('draws the accent circle behind the shape when showContainer is set', () => {
        render(<Loader showContainer />)
        expect(screen.getByRole('status').className).toMatch(/withContainer/)
    })

    it('goes through the frames and wraps around', () => {
        vi.useFakeTimers()
        const { container } = render(<Loader frameMs={100} />)
        const frames = Array.from(container.querySelectorAll('svg'))
        const activeIndex = () => frames.findIndex((f) => f.dataset.active === 'true')

        expect(activeIndex()).toBe(0)
        act(() => vi.advanceTimersByTime(100))
        expect(activeIndex()).toBe(1)
        act(() => vi.advanceTimersByTime(100 * (FRAME_COUNT - 1)))
        expect(activeIndex()).toBe(0)
    })
})
