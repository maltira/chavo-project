import {
    hasFormat,
    normalizeEntities,
    sameValue,
    toggleFormat,
    toSegments,
    trimValue,
} from './model'
import type { MessageEntity } from './model'

const e = (type: MessageEntity['type'], offset: number, length: number): MessageEntity => ({
    type,
    offset,
    length,
})

describe('normalizeEntities', () => {
    it('merges overlapping and touching ranges of one type, drops empty ones and sorts', () => {
        expect(
            normalizeEntities([
                e('bold', 6, 2),
                e('bold', 0, 4),
                e('bold', 3, 3),
                e('italic', 5, 0),
                e('underline', 0, 10),
                e('bold', 9, 1),
            ]),
        ).toEqual([e('underline', 0, 10), e('bold', 0, 8), e('bold', 9, 1)])
    })

    it('keeps different types apart', () => {
        expect(normalizeEntities([e('italic', 0, 5), e('bold', 0, 5)])).toEqual([
            e('bold', 0, 5),
            e('italic', 0, 5),
        ])
    })
})

describe('toSegments', () => {
    it('cuts the text at range boundaries and marks every part', () => {
        const parts = toSegments('привет мир', [e('bold', 0, 6), e('underline', 3, 7)])
        expect(parts.map((p) => [p.text, p.bold, p.underline])).toEqual([
            ['при', true, false],
            ['вет', true, true],
            [' мир', false, true],
        ])
        expect(parts.map((p) => p.start)).toEqual([0, 3, 6])
    })

    it('returns the whole text as one part without formatting and nothing for an empty text', () => {
        expect(toSegments('abc', [])).toEqual([
            { text: 'abc', start: 0, bold: false, italic: false, underline: false },
        ])
        expect(toSegments('', [e('bold', 0, 3)])).toEqual([])
    })

    it('ignores ranges that stick out of the text', () => {
        expect(toSegments('abc', [e('bold', 1, 10)]).map((p) => [p.text, p.bold])).toEqual([
            ['a', false],
            ['bc', true],
        ])
    })
})

describe('toggleFormat', () => {
    const base = { text: '0123456789', entities: [] as MessageEntity[] }

    it('turns a format on for a range and merges with the neighbours', () => {
        const a = toggleFormat(base, 'bold', 2, 5)
        expect(a.entities).toEqual([e('bold', 2, 3)])
        expect(toggleFormat(a, 'bold', 5, 8).entities).toEqual([e('bold', 2, 6)])
    })

    it('turns it off when the whole range already has it, splitting a larger range', () => {
        const bold = { text: base.text, entities: [e('bold', 0, 10)] }
        expect(toggleFormat(bold, 'bold', 3, 6).entities).toEqual([
            e('bold', 0, 3),
            e('bold', 6, 4),
        ])
        expect(toggleFormat(bold, 'bold', 0, 10).entities).toEqual([])
    })

    it('turns it on when only a part of the range has it', () => {
        const half = { text: base.text, entities: [e('italic', 0, 4)] }
        expect(toggleFormat(half, 'italic', 2, 8).entities).toEqual([e('italic', 0, 8)])
    })

    it('does not touch other formats and ignores an empty range', () => {
        const value = { text: base.text, entities: [e('underline', 0, 10)] }
        expect(toggleFormat(value, 'bold', 1, 3).entities).toEqual([
            e('underline', 0, 10),
            e('bold', 1, 2),
        ])
        expect(toggleFormat(value, 'bold', 4, 4)).toBe(value)
    })
})

describe('hasFormat', () => {
    it('is true only when one range covers the whole selection', () => {
        const entities = [e('bold', 0, 4), e('bold', 6, 4)]
        expect(hasFormat(entities, 'bold', 1, 3)).toBe(true)
        expect(hasFormat(entities, 'bold', 3, 7)).toBe(false)
        expect(hasFormat(entities, 'italic', 1, 3)).toBe(false)
        expect(hasFormat(entities, 'bold', 2, 2)).toBe(false)
    })
})

describe('trimValue', () => {
    it('trims the edges and shifts the ranges with the text', () => {
        const trimmed = trimValue({
            text: '  привет мир \n',
            entities: [e('bold', 2, 6), e('italic', 9, 5)],
        })
        expect(trimmed).toEqual({
            text: 'привет мир',
            entities: [e('bold', 0, 6), e('italic', 7, 3)],
        })
    })

    it('drops ranges that fall entirely into the trimmed whitespace', () => {
        expect(trimValue({ text: ' a ', entities: [e('bold', 0, 1), e('bold', 2, 1)] })).toEqual({
            text: 'a',
            entities: [],
        })
    })
})

describe('sameValue', () => {
    it('compares text and ranges regardless of how the ranges were written', () => {
        expect(
            sameValue(
                { text: 'ab', entities: [e('bold', 0, 1), e('bold', 1, 1)] },
                { text: 'ab', entities: [e('bold', 0, 2)] },
            ),
        ).toBe(true)
        expect(
            sameValue({ text: 'ab', entities: [] }, { text: 'ab', entities: [e('bold', 0, 2)] }),
        ).toBe(false)
        expect(sameValue({ text: 'ab', entities: [] }, { text: 'abc', entities: [] })).toBe(false)
    })
})
