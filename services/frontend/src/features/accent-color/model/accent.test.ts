import { applyAccent, DEFAULT_ACCENT, getInitialAccent } from './accent'

describe('accent color', () => {
    afterEach(() => {
        localStorage.clear()
        document.documentElement.style.removeProperty('--color-accent')
    })

    it('overrides the theme variable and remembers the choice', () => {
        applyAccent('#3a9ad9')
        expect(document.documentElement.style.getPropertyValue('--color-accent')).toBe('#3a9ad9')
        expect(getInitialAccent()).toBe('#3A9AD9')
    })

    it('the default color falls back to the theme', () => {
        applyAccent(DEFAULT_ACCENT)
        expect(document.documentElement.style.getPropertyValue('--color-accent')).toBe('')
    })

    it('ignores a broken stored value', () => {
        localStorage.setItem('chavo:accent', 'red')
        expect(getInitialAccent()).toBe(DEFAULT_ACCENT)
    })

    it('ignores a value that is not #RRGGBB', () => {
        applyAccent('#3a9ad9')
        applyAccent('red')
        expect(document.documentElement.style.getPropertyValue('--color-accent')).toBe('#3a9ad9')
        expect(getInitialAccent()).toBe('#3A9AD9')
    })
})
