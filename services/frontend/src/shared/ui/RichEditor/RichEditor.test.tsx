import { act, fireEvent, render, screen } from '@testing-library/react'
import { createRef } from 'react'

import { selectionOffsets, setSelectionOffsets } from '@/shared/lib'
import type { RichValue } from '@/shared/lib'

import { RichEditor } from './RichEditor'
import type { RichEditorHandle } from './RichEditor'

function setup(props: Partial<React.ComponentProps<typeof RichEditor>> = {}) {
    const ref = createRef<RichEditorHandle>()
    const onChange = vi.fn()
    const onSubmit = vi.fn()
    render(
        <RichEditor
            ref={ref}
            aria-label="Сообщение"
            placeholder="Введите сообщение"
            onChange={onChange}
            onSubmit={onSubmit}
            {...props}
        />,
    )
    const field = screen.getByRole('textbox', { name: 'Сообщение' })
    return { ref, onChange, onSubmit, field }
}

// набор текста: браузер меняет DOM и присылает input
function type(field: HTMLElement, text: string) {
    field.textContent = text
    fireEvent.input(field)
}

const last = (fn: ReturnType<typeof vi.fn>) => fn.mock.calls.at(-1)![0] as RichValue

// jsdom не шлёт selectionchange сам
function select(field: HTMLElement, start: number, end: number) {
    setSelectionOffsets(field, start, end)
    act(() => {
        document.dispatchEvent(new Event('selectionchange'))
    })
}

describe('RichEditor', () => {
    it('shows the placeholder while empty and reports typed text', () => {
        const { field, onChange } = setup()
        expect(field).toHaveAttribute('data-empty', 'true')
        expect(field).toHaveAttribute('data-placeholder', 'Введите сообщение')

        type(field, 'привет')
        expect(last(onChange)).toEqual({ text: 'привет', entities: [] })
        expect(field).toHaveAttribute('data-empty', 'false')

        // стёрли всё - мусор от браузера убран, подсказка вернулась
        field.innerHTML = '<b></b><br>'
        fireEvent.input(field)
        expect(field).toHaveAttribute('data-empty', 'true')
        expect(field.innerHTML).toBe('')
    })

    it('renders the initial value with its formatting', () => {
        const { field } = setup({
            initial: { text: 'привет мир', entities: [{ type: 'bold', offset: 0, length: 6 }] },
        })
        expect(field.querySelector('b')?.textContent).toBe('привет')
        expect(field).toHaveAttribute('data-empty', 'false')
    })

    it('Enter submits, Shift+Enter inserts a line break', () => {
        const { field, onSubmit, onChange } = setup()
        type(field, 'раз')
        setSelectionOffsets(field, 3)

        fireEvent.keyDown(field, { key: 'Enter', shiftKey: true })
        expect(onSubmit).not.toHaveBeenCalled()
        expect(last(onChange).text).toBe('раз\n')

        fireEvent.keyDown(field, { key: 'Enter' })
        expect(onSubmit).toHaveBeenCalledTimes(1)
    })

    it('does not submit while the text is being composed (IME)', () => {
        const { field, onSubmit } = setup()
        fireEvent.keyDown(field, { key: 'Enter', isComposing: true })
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('formats the selection with the hotkeys, also in the Russian layout, and toggles it off', () => {
        const { field, onChange } = setup()
        type(field, 'привет мир')
        select(field, 0, 6)

        // по физической клавише: в русской раскладке key = «и», а code остаётся KeyB
        fireEvent.keyDown(field, { key: 'и', code: 'KeyB', ctrlKey: true })
        expect(last(onChange).entities).toEqual([{ type: 'bold', offset: 0, length: 6 }])
        expect(field.querySelector('b')?.textContent).toBe('привет')
        // выделение после форматирования осталось тем же
        expect(selectionOffsets(field)).toEqual({ start: 0, end: 6 })

        fireEvent.keyDown(field, { code: 'KeyI', metaKey: true, key: 'i' })
        fireEvent.keyDown(field, { code: 'KeyU', ctrlKey: true, key: 'u' })
        expect(last(onChange).entities).toEqual([
            { type: 'bold', offset: 0, length: 6 },
            { type: 'italic', offset: 0, length: 6 },
            { type: 'underline', offset: 0, length: 6 },
        ])

        fireEvent.keyDown(field, { code: 'KeyB', ctrlKey: true, key: 'b' })
        expect(last(onChange).entities.map((e) => e.type)).toEqual(['italic', 'underline'])
    })

    it('shows the bubble over a selection, with the state of each format', () => {
        const { field, onChange } = setup({
            initial: { text: 'привет мир', entities: [{ type: 'bold', offset: 0, length: 6 }] },
        })
        expect(screen.queryByRole('toolbar', { name: 'Форматирование' })).toBeNull()

        select(field, 0, 3)
        const bubble = screen.getByRole('toolbar', { name: 'Форматирование' })
        expect(bubble).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Жирный' })).toHaveAttribute(
            'aria-pressed',
            'true',
        )
        expect(screen.getByRole('button', { name: 'Курсив' })).toHaveAttribute(
            'aria-pressed',
            'false',
        )

        // нажатие на панель не уводит фокус с поля
        const mouseDown = fireEvent.mouseDown(screen.getByRole('button', { name: 'Курсив' }))
        expect(mouseDown).toBe(false)
        fireEvent.click(screen.getByRole('button', { name: 'Курсив' }))
        expect(last(onChange).entities).toContainEqual({ type: 'italic', offset: 0, length: 3 })

        // курсор без выделения - панели нет
        select(field, 2, 2)
        expect(screen.queryByRole('toolbar', { name: 'Форматирование' })).toBeNull()
    })

    it('has no formatting for a caret without a selection and keeps the text', () => {
        const { field, onChange } = setup()
        type(field, 'abc')
        select(field, 1, 1)
        fireEvent.keyDown(field, { code: 'KeyB', ctrlKey: true, key: 'b' })
        expect(last(onChange)).toEqual({ text: 'abc', entities: [] })
    })

    it('pastes plain text with line breaks when clipboard has no rich formatting', () => {
        const { field, onChange } = setup()
        type(field, 'ac')
        setSelectionOffsets(field, 1)
        fireEvent.paste(field, {
            clipboardData: {
                getData: (type: string) => (type === 'text/plain' ? 'X\nY' : '<span>обычный</span>'),
            },
        })
        expect(last(onChange).text).toBe('aX\nYc')
        expect(field.querySelector('b')).toBeNull()
    })

    it('pastes supported formatting from HTML clipboard and drops unsafe markup', () => {
        const { field, onChange } = setup()
        type(field, 'ac')
        setSelectionOffsets(field, 1)
        fireEvent.paste(field, {
            clipboardData: {
                getData: (type: string) =>
                    type === 'text/html'
                        ? '<script>bad()</script><b>жирный</b> и <i>курсив</i>'
                        : 'жирный и курсив',
            },
        })
        expect(last(onChange)).toEqual({
            text: 'aжирный и курсивc',
            entities: [
                { type: 'bold', offset: 1, length: 6 },
                { type: 'italic', offset: 10, length: 6 },
            ],
        })
        expect(field.querySelector('script')).toBeNull()
    })

    it('inserts at the caret through the handle and keeps the caret after it', () => {
        const { ref, field, onChange } = setup()
        type(field, 'приветмир')
        select(field, 6, 6)
        act(() => ref.current!.insertText('😀'))
        expect(last(onChange).text).toBe('привет😀мир')
        expect(selectionOffsets(field)).toEqual({ start: 8, end: 8 })
    })

    it('replaces the selected text with the inserted one', () => {
        const { ref, field, onChange } = setup()
        type(field, 'abcdef')
        select(field, 1, 4)
        act(() => ref.current!.insertText('X'))
        expect(last(onChange).text).toBe('aXef')
    })

    it('remembers where the caret was when the focus has left the field', () => {
        const { ref, field, onChange } = setup()
        type(field, 'abcd')
        select(field, 2, 2)
        // фокус ушёл на кнопку вне поля: выделение сброшено
        document.getSelection()!.removeAllRanges()
        act(() => ref.current!.insertText('X'))
        expect(last(onChange).text).toBe('abXcd')
    })

    it('does not exceed the length limit and does not split an emoji', () => {
        const { ref, field, onChange } = setup({ maxLength: 5 })
        type(field, 'abc')
        select(field, 3, 3)
        act(() => ref.current!.insertText('😀😀'))
        // осталось места на две единицы UTF-16: помещается один эмодзи
        expect(last(onChange).text).toBe('abc😀')
        act(() => ref.current!.insertText('zzz'))
        expect(last(onChange).text).toBe('abc😀')
    })

    it('setValue replaces the content, the formatting and the caret', () => {
        const { ref, field, onChange } = setup()
        act(() =>
            ref.current!.setValue(
                { text: 'один два', entities: [{ type: 'italic', offset: 5, length: 3 }] },
                4,
            ),
        )
        expect(field.querySelector('i')?.textContent).toBe('два')
        expect(selectionOffsets(field)).toEqual({ start: 4, end: 4 })
        expect(last(onChange).text).toBe('один два')
        expect(ref.current!.getValue().entities).toEqual([{ type: 'italic', offset: 5, length: 3 }])
    })
})
