import { render, screen } from '@testing-library/react'

import { AvatarShape, isKnownShape } from './index'

describe('AvatarShape', () => {
  it('draws the shape behind the children with the given color', () => {
    const { container } = render(
      <AvatarShape shape="circle" color="#D97757">
        <img alt="avatar" src="" />
      </AvatarShape>,
    )
    const svg = container.querySelector('svg')
    expect(svg).toBeInTheDocument()
    expect(svg).toHaveStyle({ color: '#D97757' })
    expect(screen.getByAltText('avatar')).toBeInTheDocument()
  })

  it('renders only the children when there is no shape', () => {
    const { container } = render(
      <AvatarShape>
        <img alt="avatar" src="" />
      </AvatarShape>,
    )
    expect(container.querySelector('svg')).toBeNull()
    expect(screen.getByAltText('avatar')).toBeInTheDocument()
  })

  it('treats a key that is not in the catalog as no shape', () => {
    const { container } = render(
      <AvatarShape shape="hexagon" color="#000000">
        <img alt="avatar" src="" />
      </AvatarShape>,
    )
    expect(container.querySelector('svg')).toBeNull()
  })

  it('knows all catalog keys', () => {
    for (const key of ['circle', 'bun', '4-leaf-clover', 'pixel-circle', 'very-sunny']) {
      expect(isKnownShape(key)).toBe(true)
    }
    expect(isKnownShape('Circle')).toBe(false)
    expect(isKnownShape(null)).toBe(false)
  })
})
