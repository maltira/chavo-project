import { render } from '@testing-library/react'

import { CoverPatternIcon, isKnownCoverPattern, ProfileCover } from './index'

describe('ProfileCover', () => {
    it('renders the color halo and orbital pattern symbols for a known pattern', () => {
        const { container } = render(<ProfileCover color="#4F46E5" pattern="star" />)
        const root = container.firstElementChild
        expect(root).toHaveAttribute('data-cover-color', '#4F46E5')
        expect(root).toHaveAttribute('data-cover-pattern', 'star')
        expect(container.querySelectorAll('svg').length).toBeGreaterThan(0)
    })

    it('renders gradient-only without symbols when pattern is empty or unknown', () => {
        const empty = render(<ProfileCover color="#EC4899" pattern="" />)
        expect(empty.container.firstElementChild).toHaveAttribute('data-cover-pattern', '')
        expect(empty.container.querySelectorAll('svg')).toHaveLength(0)

        const unknown = render(<ProfileCover color="#EC4899" pattern="removed-icon" />)
        expect(unknown.container.firstElementChild).toHaveAttribute('data-cover-pattern', '')
        expect(unknown.container.querySelectorAll('svg')).toHaveLength(0)
    })

    it('knows all catalog pattern keys and renders CoverPatternIcon', () => {
        for (const key of [
            'star',
            'heart',
            'shine',
            'moon',
            'flame',
            'clover',
            'crown',
            'bolt',
            'flower',
            'music',
            'paw',
            'diamond',
        ]) {
            expect(isKnownCoverPattern(key)).toBe(true)
        }
        expect(isKnownCoverPattern('')).toBe(false)
        expect(isKnownCoverPattern('Star')).toBe(false)

        const { container } = render(<CoverPatternIcon pattern="moon" size={18} />)
        expect(container.querySelector('svg')).toBeInTheDocument()
    })
})
