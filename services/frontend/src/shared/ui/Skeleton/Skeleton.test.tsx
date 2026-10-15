import { render } from '@testing-library/react'

import { Skeleton, SkeletonRow } from './index'

describe('Skeleton', () => {
    it('makes a circle of the given size and hides it from screen readers', () => {
        const { container } = render(<Skeleton shape="circle" width={40} />)
        const node = container.firstElementChild!
        expect(node).toHaveStyle({ width: '40px', height: '40px' })
        expect(node).toHaveAttribute('aria-hidden', 'true')
    })

    it('a row has an avatar and two lines, compact - only the avatar', () => {
        const { container, rerender } = render(<SkeletonRow />)
        expect(container.querySelectorAll('[aria-hidden="true"] span[aria-hidden]')).toHaveLength(3)
        rerender(<SkeletonRow compact />)
        expect(container.querySelectorAll('[aria-hidden="true"] span[aria-hidden]')).toHaveLength(1)
    })
})
