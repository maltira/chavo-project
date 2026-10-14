import {
    COMPACT_THRESHOLD,
    COMPACT_WIDTH,
    DEFAULT_WIDTH,
    MAX_WIDTH,
    normalizeWidth,
} from './sidebarWidth'

describe('normalizeWidth', () => {
    it('keeps a width between the threshold and the maximum', () => {
        expect(normalizeWidth(300)).toBe(300)
        expect(normalizeWidth(COMPACT_THRESHOLD)).toBe(COMPACT_THRESHOLD)
        expect(normalizeWidth(MAX_WIDTH)).toBe(MAX_WIDTH)
    })

    it('limits the width from above', () => {
        expect(normalizeWidth(MAX_WIDTH + 1)).toBe(MAX_WIDTH)
        expect(normalizeWidth(5000)).toBe(MAX_WIDTH)
    })

    it('turns everything below the threshold into the compact width', () => {
        expect(normalizeWidth(COMPACT_THRESHOLD - 1)).toBe(COMPACT_WIDTH)
        expect(normalizeWidth(100)).toBe(COMPACT_WIDTH)
        expect(normalizeWidth(0)).toBe(COMPACT_WIDTH)
        expect(normalizeWidth(-50)).toBe(COMPACT_WIDTH)
    })

    it('falls back to the default for garbage', () => {
        expect(normalizeWidth(NaN)).toBe(DEFAULT_WIDTH)
        expect(normalizeWidth(Infinity)).toBe(DEFAULT_WIDTH)
    })
})
