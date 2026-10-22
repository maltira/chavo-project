import {
    useCallback,
    useEffect,
    useImperativeHandle,
    useLayoutEffect,
    useRef,
    useState,
} from 'react'
import type { ClipboardEvent, KeyboardEvent, Ref } from 'react'

import {
    cn,
    hasFormat,
    renderRich,
    selectionOffsets,
    serializeRich,
    setSelectionOffsets,
    toggleFormat,
} from '@/shared/lib'
import type { EntityType, RichValue } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './RichEditor.module.scss'

export interface RichEditorHandle {
    getValue: () => RichValue
    // программная замена содержимого (правка сообщения, очистка после отправки); курсор по умолчанию в конце
    setValue: (value: RichValue, caret?: number) => void
    // вставка в позицию курсора или поверх выделенного; фокус при этом переходит в поле
    insertText: (text: string) => void
    focus: () => void
}

interface Props {
    ref?: Ref<RichEditorHandle>
    // начальное содержимое; дальше редактор живёт сам и сообщает об изменениях через onChange
    initial?: RichValue
    placeholder?: string
    'aria-label': string
    // сколько символов (единиц UTF-16) можно ввести; лишнее не вставляется
    maxLength?: number
    onChange?: (value: RichValue) => void
    // Enter без Shift
    onSubmit?: () => void
    onEscape?: () => void
    className?: string
}

const FORMATS: { type: EntityType; icon: string; label: string; key: string }[] = [
    { type: 'bold', icon: 'font-bold', label: 'Жирный', key: 'KeyB' },
    { type: 'italic', icon: 'font-italic', label: 'Курсив', key: 'KeyI' },
    { type: 'underline', icon: 'font-underline', label: 'Подчёркнутый', key: 'KeyU' },
]

interface Bubble {
    top: number
    left: number
}

// поле ввода с форматированием: жирный, курсив и подчёркивание видны сразу. Над выделенным текстом
// появляется панель B / I / U, работают и горячие клавиши Ctrl/Cmd + B / I / U. Вставка - только
// простой текст; Enter отправляет, Shift+Enter переносит строку
export function RichEditor({
    ref,
    initial,
    placeholder,
    'aria-label': ariaLabel,
    maxLength,
    onChange,
    onSubmit,
    onEscape,
    className,
}: Props) {
    const rootRef = useRef<HTMLDivElement>(null)
    // последнее выделение внутри поля: кнопка эмодзи и панель забирают фокус, а вставлять надо в это место
    const lastRange = useRef<{ start: number; end: number } | null>(null)
    const [bubble, setBubble] = useState<Bubble | null>(null)
    const [active, setActive] = useState<Record<EntityType, boolean>>({
        bold: false,
        italic: false,
        underline: false,
    })

    const emit = useCallback(() => {
        const root = rootRef.current
        if (!root) {
            return
        }
        const value = serializeRich(root)
        // пустое поле очищаем: после удаления всего браузер оставляет мусор вроде <b></b> или <br>
        if (value.text === '' && root.innerHTML !== '') {
            root.replaceChildren()
        }
        root.dataset.empty = String(value.text === '')
        onChange?.(value)
    }, [onChange])

    // первое содержимое кладётся до отрисовки, чтобы поле не мигало пустым
    useLayoutEffect(() => {
        const root = rootRef.current
        if (root) {
            renderRich(root, initial ?? { text: '', entities: [] })
            root.dataset.empty = String((initial?.text ?? '') === '')
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps -- initial читается один раз
    }, [])

    // панель над выделением и состояние кнопок следуют за выделением
    useEffect(() => {
        function onSelectionChange() {
            const root = rootRef.current
            if (!root) {
                return
            }
            const offsets = selectionOffsets(root)
            if (!offsets) {
                setBubble(null)
                return
            }
            lastRange.current = offsets
            const value = serializeRich(root)
            if (offsets.start === offsets.end) {
                setBubble(null)
                setActive({
                    bold: queryState('bold'),
                    italic: queryState('italic'),
                    underline: queryState('underline'),
                })
                return
            }
            setActive({
                bold: hasFormat(value.entities, 'bold', offsets.start, offsets.end),
                italic: hasFormat(value.entities, 'italic', offsets.start, offsets.end),
                underline: hasFormat(value.entities, 'underline', offsets.start, offsets.end),
            })
            const range = document.getSelection()?.getRangeAt(0)
            // где разметки нет (тесты), прямоугольника нет
            const rect =
                range && typeof range.getBoundingClientRect === 'function'
                    ? range.getBoundingClientRect()
                    : null
            // панель над выделением, по центру; у краёв окна прижимается внутрь
            const center = rect ? rect.left + rect.width / 2 : 0
            setBubble({
                top: Math.max(8, (rect?.top ?? 0) - 48),
                left: Math.min(Math.max(center, 84), Math.max(84, window.innerWidth - 84)),
            })
        }
        document.addEventListener('selectionchange', onSelectionChange)
        return () => document.removeEventListener('selectionchange', onSelectionChange)
    }, [])

    // вернуть выделение туда, где оно было (после клика по кнопке вне поля)
    const restoreSelection = useCallback(() => {
        const root = rootRef.current
        if (!root) {
            return
        }
        // выделение запоминаем до фокуса: перевод фокуса может сбросить его в начало поля
        const before = selectionOffsets(root) ?? lastRange.current
        root.focus()
        if (before) {
            setSelectionOffsets(root, before.start, before.end)
        }
    }, [])

    const length = () => serializeRich(rootRef.current!).text.length

    const insert = useCallback(
        (text: string) => {
            const root = rootRef.current
            if (!root || text === '') {
                return
            }
            restoreSelection()
            const offsets = selectionOffsets(root) ?? { start: length(), end: length() }
            // место осталось только под часть: лишнее отбрасывается, эмодзи не разрезается
            let room =
                maxLength === undefined
                    ? Infinity
                    : maxLength - (length() - (offsets.end - offsets.start))
            let accepted = ''
            for (const char of text) {
                if (char.length > room) {
                    break
                }
                accepted += char
                room -= char.length
            }
            if (accepted === '') {
                return
            }
            const parts = accepted.replace(/\r\n?/g, '\n').split('\n')
            parts.forEach((part, i) => {
                if (i > 0) {
                    execOrFallbackBreak()
                }
                if (part) {
                    execOrFallbackText(part)
                }
            })
            emit()
        },
        [emit, maxLength, restoreSelection],
    )

    useImperativeHandle(
        ref,
        () => ({
            getValue: () => serializeRich(rootRef.current!),
            setValue: (value, caret = value.text.length) => {
                const root = rootRef.current
                if (!root) {
                    return
                }
                renderRich(root, value)
                root.dataset.empty = String(value.text === '')
                setSelectionOffsets(root, caret)
                lastRange.current = { start: caret, end: caret }
                onChange?.(value)
            },
            insertText: insert,
            focus: () => rootRef.current?.focus(),
        }),
        [insert, onChange],
    )

    function format(type: EntityType) {
        const root = rootRef.current
        if (!root) {
            return
        }
        restoreSelection()
        // в браузере форматирует он сам: так сохраняется история отмены (Ctrl+Z)
        if (typeof document.execCommand === 'function' && document.execCommand(type)) {
            emit()
            return
        }
        const offsets = selectionOffsets(root)
        if (!offsets || offsets.start === offsets.end) {
            return
        }
        const next = toggleFormat(serializeRich(root), type, offsets.start, offsets.end)
        renderRich(root, next)
        setSelectionOffsets(root, offsets.start, offsets.end)
        emit()
    }

    function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
        if (event.nativeEvent.isComposing) {
            return
        }
        if ((event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey) {
            // по физической клавише: работает и в русской раскладке
            const hit = FORMATS.find((f) => f.key === event.code)
            if (hit) {
                event.preventDefault()
                format(hit.type)
                return
            }
        }
        if (event.key === 'Enter') {
            event.preventDefault()
            if (event.shiftKey) {
                insert('\n')
            } else {
                onSubmit?.()
            }
            return
        }
        if (event.key === 'Escape') {
            onEscape?.()
        }
    }

    // перед вставкой проверяем предел: не больше maxLength; дальше своё через insert
    function onBeforeInput(event: InputEvent) {
        if (maxLength === undefined || !event.inputType.startsWith('insert')) {
            return
        }
        // набор через IME (композиция) прерывать нельзя - иначе ввод ломается
        if (event.inputType === 'insertCompositionText') {
            return
        }
        const root = rootRef.current
        const offsets = root && selectionOffsets(root)
        const added = event.inputType === 'insertText' ? (event.data ?? '').length : 1
        const selected = offsets ? offsets.end - offsets.start : 0
        if (length() - selected + added > maxLength) {
            event.preventDefault()
        }
    }

    useEffect(() => {
        const root = rootRef.current
        if (!root) {
            return
        }
        root.addEventListener('beforeinput', onBeforeInput)
        return () => root.removeEventListener('beforeinput', onBeforeInput)
    })

    function onPaste(event: ClipboardEvent<HTMLDivElement>) {
        // только простой текст: чужое оформление и картинки в сообщение не попадают
        event.preventDefault()
        insert(event.clipboardData.getData('text/plain'))
    }

    return (
        <>
            <div
                ref={rootRef}
                className={cn(styles.editor, className)}
                contentEditable
                suppressContentEditableWarning
                role="textbox"
                aria-multiline="true"
                aria-label={ariaLabel}
                data-placeholder={placeholder}
                data-empty="true"
                tabIndex={0}
                spellCheck
                onInput={emit}
                onKeyDown={onKeyDown}
                onPaste={onPaste}
                onDrop={(event) => {
                    // перетащенный текст вставляем простым, файлы и разметку отбрасываем
                    event.preventDefault()
                    insert(event.dataTransfer.getData('text/plain'))
                }}
            />
            {bubble && (
                <div
                    className={styles.bubble}
                    role="toolbar"
                    aria-label="Форматирование"
                    style={{ top: bubble.top, left: bubble.left }}
                    // нажатие на панель не должно снимать выделение и фокус с поля
                    onMouseDown={(event) => event.preventDefault()}
                >
                    {FORMATS.map((f) => (
                        <button
                            key={f.type}
                            type="button"
                            className={cn(styles.tool, active[f.type] && styles.toolActive)}
                            aria-label={f.label}
                            aria-pressed={active[f.type]}
                            title={`${f.label} (Ctrl+${f.key.slice(3)})`}
                            onClick={() => format(f.type)}
                        >
                            <Icon name={f.icon} size={20} color="inherit" />
                        </button>
                    ))}
                </div>
            )}
        </>
    )
}

// состояние оформления у курсора без выделения (где браузер его знает)
function queryState(command: EntityType): boolean {
    try {
        return (
            typeof document.queryCommandState === 'function' && document.queryCommandState(command)
        )
    } catch {
        return false
    }
}

// вставка текста с сохранением истории отмены; где execCommand нет (тесты), - напрямую в выделение
function execOrFallbackText(text: string) {
    if (
        typeof document.execCommand === 'function' &&
        document.execCommand('insertText', false, text)
    ) {
        return
    }
    insertNode(document.createTextNode(text))
}

function execOrFallbackBreak() {
    if (typeof document.execCommand === 'function' && document.execCommand('insertLineBreak')) {
        return
    }
    insertNode(document.createElement('br'))
}

function insertNode(node: Node) {
    const selection = document.getSelection()
    if (!selection || selection.rangeCount === 0) {
        return
    }
    const range = selection.getRangeAt(0)
    range.deleteContents()
    range.insertNode(node)
    range.setStartAfter(node)
    range.collapse(true)
    selection.removeAllRanges()
    selection.addRange(range)
}
