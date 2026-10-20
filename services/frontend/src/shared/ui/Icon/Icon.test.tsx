import { render } from '@testing-library/react'

import { Icon, isKnownIcon } from './index'

describe('Icon', () => {
    it('renders the svg by file name with the given size', () => {
        const { container } = render(<Icon name="close" size={32} />)
        const svg = container.querySelector('svg')
        expect(svg).toBeInTheDocument()
        expect(svg).toHaveAttribute('width', '32')
        expect(svg).toHaveAttribute('height', '32')
    })

    it('renders status icons by the shape- prefixed name', () => {
        for (const key of ['moon', 'shine', 'ok', 'flame', 'block', 'heart']) {
            expect(isKnownIcon(`shape-${key}`)).toBe(true)
        }
    })

    it('keeps a plain icon and a status icon with the same key apart', () => {
        expect(isKnownIcon('block')).toBe(true)
        expect(isKnownIcon('shape-block')).toBe(true)
    })

    it('renders nothing for an unknown name', () => {
        const { container } = render(<Icon name="nope" />)
        expect(container.firstChild).toBeNull()
        expect(isKnownIcon(null)).toBe(false)
    })

    it('is always hidden from assistive tech and not selectable', () => {
        const { container } = render(<Icon name="close" aria-label="Закрыть" />)
        const svg = container.querySelector('svg')
        expect(svg).toHaveAttribute('aria-hidden', 'true')
        expect(svg).toHaveStyle({ userSelect: 'none', pointerEvents: 'none' })
    })

    it('uses the theme text color by default', () => {
        const { container } = render(<Icon name="close" />)
        expect(container.querySelector('svg')).toHaveStyle({ color: 'var(--color-text)' })
    })

    it('takes a custom color and keeps other styles', () => {
        const { container } = render(<Icon name="heart" color="#D97757" style={{ opacity: 0.5 }} />)
        expect(container.querySelector('svg')).toHaveStyle({ color: '#D97757', opacity: '0.5' })
    })
})
