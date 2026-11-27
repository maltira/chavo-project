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
    CODE_LANGUAGES,
    findLinkInRange,
    findPreLanguageInRange,
    fromHtml,
    hasFormat,
    isSafeHttpUrl,
    normalizeEntities,
    renderRich,
    replaceRangeWithMention,
    selectionOffsets,
    serializeRich,
    setLink,
    setPre,
    setSelectionOffsets,
    toggleFormat,
} from '@/shared/lib'
import type { MessageEntity, RichValue, ToggleableFormat } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './RichEditor.module.scss'

export interface RichEditorHandle {
    getValue: () => RichValue
    // программная замена содержимого (правка сообщения, очистка после отправки); курсор по умолчанию в конце
    setValue: (value: RichValue, caret?: number) => void
    // вставка в позицию курсора или поверх выделенного; фокус при этом переходит в поле
    insertText: (text: string) => void
    // замена диапазона (например @query) упоминанием пользователя
    insertMention: (start: number, end: number, label: string, userId: string) => void
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
    // позиция курсора (или null при выделении/расфокусе) - используется для автодополнения @
    onCaretChange?: (value: RichValue, caret: number | null) => void
    // перехват клавиш (например навигация в списке упоминаний); true - событие обработано
    onSpecialKeyDown?: (event: KeyboardEvent<HTMLDivElement>) => boolean
    // Enter без Shift
    onSubmit?: () => void
    onEscape?: () => void
    className?: string
}

interface FormatTool {
    type: ToggleableFormat
    icon: string
    label: string
    code: string
    shift?: boolean
    hint: string
}

const FORMATS: FormatTool[] = [
    { type: 'bold', icon: 'font-bold', label: 'Жирный', code: 'KeyB', hint: 'Ctrl+B' },
    { type: 'italic', icon: 'font-italic', label: 'Курсив', code: 'KeyI', hint: 'Ctrl+I' },
    { type: 'underline', icon: 'font-underline', label: 'Подчёркнутый', code: 'KeyU', hint: 'Ctrl+U' },
    {
        type: 'strikethrough',
        icon: 'font-strikethrough',
        label: 'Зачёркнутый',
        code: 'KeyX',
        shift: true,
        hint: 'Ctrl+Shift+X',
    },
    {
        type: 'code',
        icon: 'font-code',
        label: 'Код',
        code: 'KeyM',
        shift: true,
        hint: 'Ctrl+Shift+M',
    },
    {
        type: 'spoiler',
        icon: 'eye-closed',
        label: 'Спойлер',
        code: 'KeyP',
        shift: true,
        hint: 'Ctrl+Shift+P',
    },
    {
        type: 'blockquote',
        icon: 'font-quote',
        label: 'Цитата',
        code: 'Period',
        shift: true,
        hint: 'Ctrl+Shift+.',
    },
]

const NATIVE_COMMANDS = new Set<ToggleableFormat>(['bold', 'italic', 'underline'])

interface Bubble {
    top: number
    left: number
}

const EMPTY_ACTIVE: Record<ToggleableFormat, boolean> = {
    bold: false,
    italic: false,
    underline: false,
    strikethrough: false,
    code: false,
    pre: false,
    spoiler: false,
    blockquote: false,
}

// поле ввода с форматированием: жирный, курсив, подчёркивание, зачёркнутый, код, спойлер, цитата и ссылка.
// Над выделенным текстом появляется панель инструментов, работают горячие клавиши. Enter отправляет,
// Shift+Enter переносит строку
export function RichEditor({
    ref,
    initial,
    placeholder,
    'aria-label': ariaLabel,
    maxLength,
    onChange,
    onCaretChange,
    onSpecialKeyDown,
    onSubmit,
    onEscape,
    className,
}: Props) {
    const rootRef = useRef<HTMLDivElement>(null)
    // последнее выделение внутри поля: кнопка эмодзи и панель забирают фокус, а вставлять надо в это место
    const lastRange = useRef<{ start: number; end: number } | null>(null)
    const [bubble, setBubble] = useState<Bubble | null>(null)
    const [active, setActive] = useState<Record<ToggleableFormat, boolean>>(EMPTY_ACTIVE)
    const [activeLink, setActiveLink] = useState<string | null>(null)
    const [activePreLang, setActivePreLang] = useState<string | null>(null)
    // встроенная форма ввода ссылки в плавающей панели
    const [linkEditing, setLinkEditing] = useState(false)
    const [linkInput, setLinkInput] = useState('')
    const [linkError, setLinkError] = useState(false)
    const linkRangeRef = useRef<{ start: number; end: number } | null>(null)
    const linkInputRef = useRef<HTMLInputElement>(null)
    // встроенная форма выбора языка блока кода (pre)
    const [preEditing, setPreEditing] = useState(false)
    const [preLang, setPreLang] = useState('')
    const preRangeRef = useRef<{ start: number; end: number } | null>(null)
    const preSelectRef = useRef<HTMLSelectElement>(null)

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
        const offsets = selectionOffsets(root)
        onCaretChange?.(value, offsets && offsets.start === offsets.end ? offsets.start : null)
    }, [onCaretChange, onChange])

    // первое содержимое кладётся до отрисовки, чтобы поле не мигало пустым
    useLayoutEffect(() => {
        const root = rootRef.current
        if (root) {
            renderRich(root, initial ?? { text: '', entities: [] })
            root.dataset.empty = String((initial?.text ?? '') === '')
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps -- initial читается один раз
    }, [])

    useEffect(() => {
        if (linkEditing) {
            linkInputRef.current?.focus()
            linkInputRef.current?.select()
        }
    }, [linkEditing])

    useEffect(() => {
        if (preEditing) {
            preSelectRef.current?.focus()
        }
    }, [preEditing])

    // панель над выделением и состояние кнопок следуют за выделением
    useEffect(() => {
        function onSelectionChange() {
            const root = rootRef.current
            if (!root) {
                return
            }
            // пока пользователь вводит URL или выбирает язык кода, панель не прячем
            if (linkEditing || preEditing) {
                return
            }
            const offsets = selectionOffsets(root)
            if (!offsets) {
                setBubble(null)
                onCaretChange?.(serializeRich(root), null)
                return
            }
            lastRange.current = offsets
            const value = serializeRich(root)
            if (offsets.start === offsets.end) {
                setBubble(null)
                setActiveLink(null)
                setActivePreLang(null)
                setActive({
                    ...EMPTY_ACTIVE,
                    bold: queryState('bold'),
                    italic: queryState('italic'),
                    underline: queryState('underline'),
                })
                onCaretChange?.(value, offsets.start)
                return
            }
            onCaretChange?.(value, null)
            setActive({
                bold: hasFormat(value.entities, 'bold', offsets.start, offsets.end),
                italic: hasFormat(value.entities, 'italic', offsets.start, offsets.end),
                underline: hasFormat(value.entities, 'underline', offsets.start, offsets.end),
                strikethrough: hasFormat(
                    value.entities,
                    'strikethrough',
                    offsets.start,
                    offsets.end,
                ),
                code: hasFormat(value.entities, 'code', offsets.start, offsets.end),
                pre: hasFormat(value.entities, 'pre', offsets.start, offsets.end),
                spoiler: hasFormat(value.entities, 'spoiler', offsets.start, offsets.end),
                blockquote: hasFormat(value.entities, 'blockquote', offsets.start, offsets.end),
            })
            setActiveLink(findLinkInRange(value.entities, offsets.start, offsets.end))
            setActivePreLang(findPreLanguageInRange(value.entities, offsets.start, offsets.end))
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
                left: Math.min(Math.max(center, 184), Math.max(184, window.innerWidth - 184)),
            })
        }
        document.addEventListener('selectionchange', onSelectionChange)
        return () => document.removeEventListener('selectionchange', onSelectionChange)
    }, [linkEditing, preEditing, onCaretChange])

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

    const insertMention = useCallback(
        (start: number, end: number, label: string, userId: string) => {
            const root = rootRef.current
            if (!root) {
                return
            }
            const current = serializeRich(root)
            const nextLen = current.text.length - (end - start) + label.length + 1
            if (maxLength !== undefined && nextLen > maxLength) {
                return
            }
            const { value, caret } = replaceRangeWithMention(current, start, end, label, userId)
            renderRich(root, value)
            root.dataset.empty = String(value.text === '')
            root.focus()
            setSelectionOffsets(root, caret)
            lastRange.current = { start: caret, end: caret }
            onChange?.(value)
            onCaretChange?.(value, caret)
        },
        [maxLength, onCaretChange, onChange],
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
                setLinkEditing(false)
                setPreEditing(false)
                renderRich(root, value)
                root.dataset.empty = String(value.text === '')
                setSelectionOffsets(root, caret)
                lastRange.current = { start: caret, end: caret }
                onChange?.(value)
                onCaretChange?.(value, caret)
            },
            insertText: insert,
            insertMention,
            focus: () => rootRef.current?.focus(),
        }),
        [insert, insertMention, onCaretChange, onChange],
    )

    function format(type: ToggleableFormat) {
        const root = rootRef.current
        if (!root) {
            return
        }
        restoreSelection()
        // для базовых форматов в браузере используем execCommand, чтобы работала история Ctrl+Z
        if (
            NATIVE_COMMANDS.has(type) &&
            typeof document.execCommand === 'function' &&
            document.execCommand(type)
        ) {
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

    function openLinkEditor() {
        const root = rootRef.current
        if (!root) {
            return
        }
        const offsets = selectionOffsets(root) ?? lastRange.current
        if (!offsets || offsets.start === offsets.end) {
            return
        }
        const existing = findLinkInRange(serializeRich(root).entities, offsets.start, offsets.end)
        linkRangeRef.current = offsets
        setLinkInput(existing ?? 'https://')
        setLinkError(false)
        setLinkEditing(true)
    }

    function closeLinkEditor() {
        setLinkEditing(false)
        setLinkError(false)
        const root = rootRef.current
        const saved = linkRangeRef.current
        if (root && saved) {
            root.focus()
            setSelectionOffsets(root, saved.start, saved.end)
        }
    }

    function applyLink(rawUrl: string | null) {
        const root = rootRef.current
        const saved = linkRangeRef.current
        if (!root || !saved) {
            setLinkEditing(false)
            return
        }
        if (rawUrl !== null) {
            const trimmed = rawUrl.trim()
            if (!trimmed) {
                rawUrl = null
            } else {
                const candidate = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
                if (!isSafeHttpUrl(candidate)) {
                    setLinkError(true)
                    return
                }
                rawUrl = candidate
            }
        }
        const next = setLink(serializeRich(root), saved.start, saved.end, rawUrl)
        setLinkEditing(false)
        setLinkError(false)
        renderRich(root, next)
        root.focus()
        setSelectionOffsets(root, saved.start, saved.end)
        lastRange.current = saved
        emit()
    }

    function openPreEditor() {
        const root = rootRef.current
        if (!root) {
            return
        }
        const offsets = selectionOffsets(root) ?? lastRange.current
        if (!offsets || offsets.start === offsets.end) {
            return
        }
        const existing = findPreLanguageInRange(
            serializeRich(root).entities,
            offsets.start,
            offsets.end,
        )
        preRangeRef.current = offsets
        setPreLang(existing ?? '')
        setLinkEditing(false)
        setPreEditing(true)
    }

    function closePreEditor() {
        setPreEditing(false)
        const root = rootRef.current
        const saved = preRangeRef.current
        if (root && saved) {
            root.focus()
            setSelectionOffsets(root, saved.start, saved.end)
        }
    }

    function applyPre(language: string | null) {
        const root = rootRef.current
        const saved = preRangeRef.current
        if (!root || !saved) {
            setPreEditing(false)
            return
        }
        const next = setPre(serializeRich(root), saved.start, saved.end, language)
        setPreEditing(false)
        renderRich(root, next)
        root.focus()
        setSelectionOffsets(root, saved.start, saved.end)
        lastRange.current = saved
        emit()
    }

    function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
        if (event.nativeEvent.isComposing) {
            return
        }
        if (onSpecialKeyDown?.(event)) {
            return
        }
        if ((event.ctrlKey || event.metaKey) && !event.altKey) {
            // Ctrl/Cmd + K - вставить или изменить ссылку на выделенном тексте
            if (!event.shiftKey && event.code === 'KeyK') {
                const offsets = rootRef.current ? selectionOffsets(rootRef.current) : null
                if (offsets && offsets.start !== offsets.end) {
                    event.preventDefault()
                    openLinkEditor()
                }
                return
            }
            // Ctrl/Cmd + Shift + C - оформить блок кода с выбором языка
            if (event.shiftKey && event.code === 'KeyC') {
                const offsets = rootRef.current ? selectionOffsets(rootRef.current) : null
                if (offsets && offsets.start !== offsets.end) {
                    event.preventDefault()
                    openPreEditor()
                }
                return
            }
            // по физической клавише: работает и в русской раскладке
            const hit = FORMATS.find((f) => f.code === event.code && !!f.shift === event.shiftKey)
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

    const insertRich = useCallback(
        (incoming: RichValue) => {
            const root = rootRef.current
            if (!root || incoming.text === '') {
                return
            }
            restoreSelection()
            const current = serializeRich(root)
            const offsets = selectionOffsets(root) ?? {
                start: current.text.length,
                end: current.text.length,
            }
            let room =
                maxLength === undefined
                    ? Infinity
                    : maxLength - (current.text.length - (offsets.end - offsets.start))
            let accepted = ''
            for (const char of incoming.text.replace(/\r\n?/g, '\n')) {
                if (char.length > room) {
                    break
                }
                accepted += char
                room -= char.length
            }
            if (accepted === '') {
                return
            }
            const insertedLen = accepted.length
            const delta = insertedLen - (offsets.end - offsets.start)
            const mergedEntities: MessageEntity[] = []
            // сохраняем существующие диапазоны до и после заменённого фрагмента
            for (const e of current.entities) {
                const from = e.offset
                const to = e.offset + e.length
                if (to <= offsets.start) {
                    mergedEntities.push(e)
                } else if (from >= offsets.end) {
                    mergedEntities.push({ ...e, offset: from + delta })
                } else {
                    if (from < offsets.start) {
                        mergedEntities.push({
                            ...e,
                            offset: from,
                            length: offsets.start - from,
                        })
                    }
                    if (to > offsets.end) {
                        mergedEntities.push({
                            ...e,
                            offset: offsets.start + insertedLen,
                            length: to - offsets.end,
                        })
                    }
                }
            }
            // добавляем диапазоны вставленного фрагмента (обрезанные по принятой длине)
            for (const e of incoming.entities) {
                const from = Math.max(0, e.offset)
                const to = Math.min(insertedLen, e.offset + e.length)
                if (to > from) {
                    mergedEntities.push({
                        ...e,
                        offset: offsets.start + from,
                        length: to - from,
                    })
                }
            }
            const next: RichValue = {
                text: current.text.slice(0, offsets.start) + accepted + current.text.slice(offsets.end),
                entities: normalizeEntities(mergedEntities),
            }
            renderRich(root, next)
            setSelectionOffsets(root, offsets.start + insertedLen)
            emit()
        },
        [emit, maxLength, restoreSelection],
    )

    function onPaste(event: ClipboardEvent<HTMLDivElement>) {
        event.preventDefault()
        const plain = event.clipboardData.getData('text/plain')
        // голый URL (например скопированный из <code> в чате или документации) вставляем обычной ссылкой, а не кодом
        if (isSafeHttpUrl(plain.trim())) {
            insert(plain)
            return
        }
        const html = event.clipboardData.getData('text/html')
        if (html) {
            const parsed = fromHtml(html)
            if (parsed.entities.length > 0) {
                insertRich(parsed)
                return
            }
        }
        insert(plain)
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
                    // нажатие на кнопки панели не должно снимать выделение и фокус с поля
                    onMouseDown={(event) => {
                        if (!linkEditing && !preEditing) {
                            event.preventDefault()
                        }
                    }}
                >
                    {linkEditing ? (
                        <div className={styles.linkForm}>
                            <input
                                ref={linkInputRef}
                                type="url"
                                className={cn(styles.linkInput, linkError && styles.linkInputError)}
                                value={linkInput}
                                placeholder="https://example.com"
                                aria-label="Адрес ссылки"
                                onChange={(event) => {
                                    setLinkInput(event.target.value)
                                    setLinkError(false)
                                }}
                                onKeyDown={(event) => {
                                    if (event.key === 'Enter') {
                                        event.preventDefault()
                                        applyLink(linkInput)
                                    } else if (event.key === 'Escape') {
                                        event.preventDefault()
                                        closeLinkEditor()
                                    }
                                }}
                            />
                            <button
                                type="button"
                                className={styles.tool}
                                aria-label="Применить ссылку"
                                onClick={() => applyLink(linkInput)}
                            >
                                <Icon name="check" size={18} color="inherit" />
                            </button>
                            {activeLink && (
                                <button
                                    type="button"
                                    className={styles.tool}
                                    aria-label="Убрать ссылку"
                                    onClick={() => applyLink(null)}
                                >
                                    <Icon name="trash" size={18} color="inherit" />
                                </button>
                            )}
                            <button
                                type="button"
                                className={styles.tool}
                                aria-label="Отменить ввод ссылки"
                                onClick={closeLinkEditor}
                            >
                                <Icon name="close" size={18} color="inherit" />
                            </button>
                        </div>
                    ) : preEditing ? (
                        <div className={styles.linkForm}>
                            <select
                                ref={preSelectRef}
                                className={styles.langSelect}
                                aria-label="Язык кода"
                                value={preLang}
                                onChange={(event) => setPreLang(event.target.value)}
                                onKeyDown={(event) => {
                                    if (event.key === 'Enter') {
                                        event.preventDefault()
                                        applyPre(preLang)
                                    } else if (event.key === 'Escape') {
                                        event.preventDefault()
                                        closePreEditor()
                                    }
                                }}
                            >
                                {CODE_LANGUAGES.map((lang) => (
                                    <option key={lang.value || 'plain'} value={lang.value}>
                                        {lang.label}
                                    </option>
                                ))}
                            </select>
                            <button
                                type="button"
                                className={styles.tool}
                                aria-label="Применить блок кода"
                                onClick={() => applyPre(preLang)}
                            >
                                <Icon name="check" size={18} color="inherit" />
                            </button>
                            {activePreLang !== null && (
                                <button
                                    type="button"
                                    className={styles.tool}
                                    aria-label="Убрать блок кода"
                                    onClick={() => applyPre(null)}
                                >
                                    <Icon name="trash" size={18} color="inherit" />
                                </button>
                            )}
                            <button
                                type="button"
                                className={styles.tool}
                                aria-label="Отменить выбор языка"
                                onClick={closePreEditor}
                            >
                                <Icon name="close" size={18} color="inherit" />
                            </button>
                        </div>
                    ) : (
                        <>
                            {FORMATS.map((f) => (
                                <button
                                    key={f.type}
                                    type="button"
                                    className={cn(styles.tool, active[f.type] && styles.toolActive)}
                                    aria-label={f.label}
                                    aria-pressed={active[f.type]}
                                    title={`${f.label} (${f.hint})`}
                                    onClick={() => format(f.type)}
                                >
                                    <Icon name={f.icon} size={20} color="inherit" />
                                </button>
                            ))}
                            <button
                                type="button"
                                className={cn(
                                    styles.tool,
                                    activePreLang !== null && styles.toolActive,
                                )}
                                aria-label="Блок кода"
                                aria-pressed={activePreLang !== null}
                                title="Блок кода (Ctrl+Shift+C)"
                                onClick={openPreEditor}
                            >
                                <Icon name="font-code-block" size={20} color="inherit" />
                            </button>
                            <button
                                type="button"
                                className={cn(styles.tool, !!activeLink && styles.toolActive)}
                                aria-label="Ссылка"
                                aria-pressed={!!activeLink}
                                title="Ссылка (Ctrl+K)"
                                onClick={openLinkEditor}
                            >
                                <Icon name="link" size={20} color="inherit" />
                            </button>
                        </>
                    )}
                </div>
            )}
        </>
    )
}

// состояние оформления у курсора без выделения (где браузер его знает)
function queryState(command: ToggleableFormat): boolean {
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
