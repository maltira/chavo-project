import { fromHtml, render, selectionOffsets, serialize, setSelectionOffsets, toHtml } from './dom'
import type { MessageEntity } from './model'

const e = (type: MessageEntity['type'], offset: number, length: number): MessageEntity => ({
    type,
    offset,
    length,
})

function editor(html = '') {
    const root = document.createElement('div')
    root.contentEditable = 'true'
    root.innerHTML = html
    document.body.appendChild(root)
    return root
}

afterEach(() => {
    document.body.innerHTML = ''
})

describe('serialize', () => {
    it('reads text and the formatting of b, i, u', () => {
        const root = editor('<b>при</b>вет <i><u>мир</u></i>')
        expect(serialize(root)).toEqual({
            text: 'привет мир',
            entities: [e('bold', 0, 3), e('italic', 7, 3), e('underline', 7, 3)],
        })
    })

    it('understands the tags and styles a browser may insert', () => {
        const root = editor(
            '<strong>a</strong><em>b</em><span style="font-weight: 700">c</span><span style="font-style: italic">d</span><span style="text-decoration: underline">e</span>',
        )
        expect(serialize(root)).toEqual({
            text: 'abcde',
            entities: [
                e('bold', 0, 1),
                e('italic', 1, 1),
                e('bold', 2, 1),
                e('italic', 3, 1),
                e('underline', 4, 1),
            ],
        })
    })

    it('merges the same formatting split across several nodes', () => {
        expect(serialize(editor('<b>a</b><b>b</b>c<b>d</b>')).entities).toEqual([
            e('bold', 0, 2),
            e('bold', 3, 1),
        ])
    })

    it('counts <br> as a new line and ignores the service one at the very end', () => {
        expect(serialize(editor('a<br>b')).text).toBe('a\nb')
        // браузер ставит служебный <br> после последнего перевода строки: он не часть текста
        expect(serialize(editor('a<br><br>')).text).toBe('a\n')
        expect(serialize(editor('<br>')).text).toBe('')
        expect(serialize(editor('')).text).toBe('')
    })

    it('keeps the formatting going through a line break', () => {
        expect(serialize(editor('<b>a<br>b</b>')).entities).toEqual([e('bold', 0, 3)])
    })

    it('turns non-breaking spaces into spaces and starts blocks on a new line', () => {
        expect(serialize(editor('a&nbsp;&nbsp;b')).text).toBe('a  b')
        expect(serialize(editor('a<div>b</div><div>c</div>')).text).toBe('a\nb\nc')
    })

    it('does not take an emoji apart: indexes are UTF-16 units', () => {
        const root = editor('a<b>😀</b>b')
        expect(serialize(root)).toEqual({ text: 'a😀b', entities: [e('bold', 1, 2)] })
    })
})

describe('render', () => {
    it('writes the value back so that serialize gives the same value', () => {
        const root = editor()
        const value = {
            text: 'привет\nмир и всё',
            entities: [e('bold', 0, 6), e('italic', 3, 8), e('underline', 7, 3)],
        }
        render(root, value)
        expect(serialize(root)).toEqual(value)
        expect(root.querySelectorAll('br')).toHaveLength(1)
        // жирный текст разбит по границам других оформлений, но весь он в <b>
        expect(Array.from(root.querySelectorAll('b'), (b) => b.textContent).join('')).toBe('привет')
    })

    it('keeps a trailing line break reachable by the caret', () => {
        const root = editor()
        render(root, { text: 'a\n', entities: [] })
        expect(serialize(root).text).toBe('a\n')
        expect(root.querySelectorAll('br')).toHaveLength(2)
    })

    it('renders an empty value as an empty editor', () => {
        const root = editor('<b>x</b>')
        render(root, { text: '', entities: [] })
        expect(root.innerHTML).toBe('')
    })
})

describe('selection offsets', () => {
    it('reports the selection in text indexes, across formatting and line breaks', () => {
        const root = editor()
        render(root, { text: 'ab\ncd', entities: [e('bold', 1, 3)] })
        setSelectionOffsets(root, 1, 4)
        expect(selectionOffsets(root)).toEqual({ start: 1, end: 4 })
        setSelectionOffsets(root, 3)
        expect(selectionOffsets(root)).toEqual({ start: 3, end: 3 })
        setSelectionOffsets(root, 5)
        expect(selectionOffsets(root)).toEqual({ start: 5, end: 5 })
    })

    it('puts the caret after a trailing line break', () => {
        const root = editor()
        render(root, { text: 'a\n', entities: [] })
        setSelectionOffsets(root, 2)
        expect(selectionOffsets(root)).toEqual({ start: 2, end: 2 })
    })

    it('is null when the selection is outside the editor', () => {
        const root = editor('abc')
        const other = document.createElement('p')
        other.textContent = 'outside'
        document.body.appendChild(other)
        const range = document.createRange()
        range.selectNodeContents(other)
        document.getSelection()!.removeAllRanges()
        document.getSelection()!.addRange(range)
        expect(selectionOffsets(root)).toBeNull()
    })
})

describe('toHtml / fromHtml', () => {
    it('round-trips formatted text with escaping and line breaks', () => {
        const value = {
            text: '<привет>\nмир',
            entities: [e('bold', 0, 8), e('italic', 9, 3), e('underline', 9, 3)],
        }
        const html = toHtml(value)
        expect(html).toBe('<b>&lt;привет&gt;</b><br><i><u>мир</u></i>')
        expect(fromHtml(html)).toEqual(value)
    })

    it('drops unsafe tags like script and style when parsing pasted HTML', () => {
        const parsed = fromHtml('<script>alert(1)</script><strong>жирный</strong><style>x{}</style>')
        expect(parsed).toEqual({
            text: 'жирный',
            entities: [e('bold', 0, 6)],
        })
    })
})
