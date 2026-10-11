import { render, screen } from '@testing-library/react'

import { HomePage } from '../index'

describe('HomePage', () => {
  it('shows the product name', () => {
    render(<HomePage />)
    expect(screen.getByRole('heading', { name: 'Chavo' })).toBeInTheDocument()
  })
})
