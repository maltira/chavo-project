import { fireEvent, render, screen } from '@testing-library/react'

import { applyTheme, getInitialTheme, ThemeToggle } from '../index'

describe('toggle-theme', () => {
    beforeEach(() => {
        localStorage.clear()
        delete document.documentElement.dataset.theme
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('is dark by default and remembers the choice', () => {
        expect(getInitialTheme()).toBe('dark')
        applyTheme('light')
        expect(document.documentElement.dataset.theme).toBe('light')
        expect(getInitialTheme()).toBe('light')
    })

    it('falls back to dark when storage is unavailable', () => {
        vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('blocked')
        })
        vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('blocked')
        })
        expect(getInitialTheme()).toBe('dark')
        expect(() => applyTheme('light')).not.toThrow()
        expect(document.documentElement.dataset.theme).toBe('light')
    })

    it('switches the theme on click', () => {
        applyTheme('dark')
        render(<ThemeToggle />)

        fireEvent.click(screen.getByRole('button', { name: 'Включить светлую тему' }))
        expect(document.documentElement.dataset.theme).toBe('light')
        expect(screen.getByRole('button', { name: 'Включить тёмную тему' })).toBeInTheDocument()
    })
})
