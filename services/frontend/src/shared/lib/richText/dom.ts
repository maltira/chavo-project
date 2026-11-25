import { normalizeEntities, toSegments } from './model'
import type { EntityType, MessageEntity, RichValue } from './model'

// связь редактора (contenteditable) с моделью «текст + диапазоны». В DOM допустимы <b>, <i>, <u>, переводы
// строки <br> и обычный текст; всё остальное, что вставил браузер (<strong>, <em>, <span style>, <div>),
// читается так же

interface Flags {
    bold: boolean
    italic: boolean
    underline: boolean
}

interface Run extends Flags {
    start: number
    end: number
}

const BLOCKS = new Set(['DIV', 'P', 'LI', 'BLOCKQUOTE', 'H1', 'H2', 'H3'])
const IGNORED = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE', 'HEAD', 'TITLE', 'META'])

function flagsOf(el: HTMLElement, parent: Flags): Flags {
    const flags = { ...parent }
    switch (el.nodeName) {
        case 'B':
        case 'STRONG':
            flags.bold = true
            break
        case 'I':
        case 'EM':
            flags.italic = true
            break
        case 'U':
            flags.underline = true
            break
    }
    // оформление через style: так вставляет текст часть браузеров
    const weight = el.style?.fontWeight
    if (weight === 'bold' || weight === 'bolder' || Number(weight) >= 600) {
        flags.bold = true
    }
    if (el.style?.fontStyle === 'italic') {
        flags.italic = true
    }
    if (
        el.style?.textDecorationLine?.includes('underline') ||
        el.style?.textDecoration?.includes('underline')
    ) {
        flags.underline = true
    }
    return flags
}

// последний <br> в конце редактора - служебный: браузер ставит его, чтобы показать пустую последнюю строку
function trailingBreak(root: Node): Node | null {
    let node: Node = root
    while (node.lastChild) {
        node = node.lastChild
    }
    return node.nodeName === 'BR' && node !== root ? node : null
}

// keepTrailingBreak - считать и служебный <br> (нужно, когда читается только часть содержимого)
function read(root: Node, keepTrailingBreak = false): { text: string; runs: Run[] } {
    const skip = keepTrailingBreak ? null : trailingBreak(root)
    let text = ''
    const runs: Run[] = []

    const push = (chunk: string, flags: Flags) => {
        if (chunk === '') {
            return
        }
        runs.push({ ...flags, start: text.length, end: text.length + chunk.length })
        text += chunk
    }
    const walk = (node: Node, flags: Flags) => {
        if (node.nodeType === Node.TEXT_NODE) {
            // неразрывный пробел браузер подставляет сам (двойные пробелы, пробел в конце)
            push((node as Text).data.replace(/\u00a0/g, ' '), flags)
            return
        }
        if (node.nodeType !== Node.ELEMENT_NODE) {
            return
        }
        const el = node as HTMLElement
        if (IGNORED.has(el.nodeName)) {
            return
        }
        if (el.nodeName === 'BR') {
            if (el !== skip) {
                push('\n', flags)
            }
            return
        }
        const own = flagsOf(el, flags)
        // блок (вставленный браузером абзац) начинается с новой строки
        if (BLOCKS.has(el.nodeName) && text !== '' && !text.endsWith('\n')) {
            push('\n', own)
        }
        el.childNodes.forEach((child) => walk(child, own))
    }
    root.childNodes.forEach((child) =>
        walk(child, { bold: false, italic: false, underline: false }),
    )
    return { text, runs }
}

// склеивает подряд идущие участки с одним оформлением в диапазоны
function entitiesOf(runs: Run[]): MessageEntity[] {
    const entities: MessageEntity[] = []
    for (const type of ['bold', 'italic', 'underline'] as EntityType[]) {
        for (const run of runs) {
            if (run[type]) {
                entities.push({ type, offset: run.start, length: run.end - run.start })
            }
        }
    }
    return normalizeEntities(entities)
}

// текст и разметка редактора
export function serialize(root: Node): RichValue {
    const { text, runs } = read(root)
    return { text, entities: entitiesOf(runs) }
}

// длина текста до границы (node, offset): нужна, чтобы узнать, где в тексте курсор
function textBefore(root: Node, node: Node, offset: number): number {
    const range = document.createRange()
    range.setStart(root, 0)
    range.setEnd(node, offset)
    const holder = document.createElement('div')
    holder.appendChild(range.cloneContents())
    return read(holder, true).text.length
}

// выделение в индексах текста; null - выделения нет или оно вне редактора
export function selectionOffsets(root: Node): { start: number; end: number } | null {
    const selection = document.getSelection()
    if (!selection || selection.rangeCount === 0) {
        return null
    }
    const range = selection.getRangeAt(0)
    if (!root.contains(range.startContainer) || !root.contains(range.endContainer)) {
        return null
    }
    return {
        start: textBefore(root, range.startContainer, range.startOffset),
        end: textBefore(root, range.endContainer, range.endOffset),
    }
}

// место в DOM для индекса текста: конец предыдущего узла важнее начала следующего
function locate(root: Node, target: number): { node: Node; offset: number } {
    const skip = trailingBreak(root)
    let count = 0
    let found: { node: Node; offset: number } | null = null

    const walk = (node: Node): boolean => {
        if (node.nodeType === Node.TEXT_NODE) {
            const length = (node as Text).data.length
            if (target <= count + length) {
                found = { node, offset: target - count }
                return true
            }
            count += length
            return false
        }
        if (node.nodeName === 'BR') {
            if (node === skip) {
                return false
            }
            const parent = node.parentNode!
            const index = Array.prototype.indexOf.call(parent.childNodes, node) as number
            if (target === count) {
                found = { node: parent, offset: index }
                return true
            }
            count += 1
            if (target === count) {
                found = { node: parent, offset: index + 1 }
                return true
            }
            return false
        }
        for (const child of Array.from(node.childNodes)) {
            if (walk(child)) {
                return true
            }
        }
        return false
    }
    walk(root)
    return found ?? { node: root, offset: root.childNodes.length }
}

export function setSelectionOffsets(root: Node, start: number, end = start) {
    const selection = document.getSelection()
    if (!selection) {
        return
    }
    const from = locate(root, start)
    const to = start === end ? from : locate(root, end)
    const range = document.createRange()
    range.setStart(from.node, from.offset)
    range.setEnd(to.node, to.offset)
    selection.removeAllRanges()
    selection.addRange(range)
}

// кладёт в редактор текст с разметкой: переводы строк - <br>, оформление - <b>, <i>, <u>
export function render(root: HTMLElement, value: RichValue) {
    root.replaceChildren()
    for (const segment of toSegments(value.text, value.entities)) {
        const parts = segment.text.split('\n')
        const content = document.createDocumentFragment()
        parts.forEach((part, i) => {
            if (i > 0) {
                content.appendChild(document.createElement('br'))
            }
            if (part) {
                content.appendChild(document.createTextNode(part))
            }
        })
        let node: Node = content
        for (const [on, tag] of [
            [segment.underline, 'u'],
            [segment.italic, 'i'],
            [segment.bold, 'b'],
        ] as const) {
            if (on) {
                const wrapper = document.createElement(tag)
                wrapper.appendChild(node)
                node = wrapper
            }
        }
        root.appendChild(node)
    }
    // текст оканчивается переводом строки - нужна служебная строка, чтобы курсор встал после него
    if (value.text.endsWith('\n')) {
        root.appendChild(document.createElement('br'))
    }
}

function escapeHtml(text: string): string {
    return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// собирает чистый HTML с <b>, <i>, <u> и <br> для буфера обмена
export function toHtml(value: RichValue): string {
    return toSegments(value.text, value.entities)
        .map((segment) => {
            let chunk = escapeHtml(segment.text).replace(/\n/g, '<br>')
            if (segment.underline) {
                chunk = `<u>${chunk}</u>`
            }
            if (segment.italic) {
                chunk = `<i>${chunk}</i>`
            }
            if (segment.bold) {
                chunk = `<b>${chunk}</b>`
            }
            return chunk
        })
        .join('')
}

// разбирает HTML из буфера обмена в текст + поддерживаемые диапазоны; небезопасные теги отбрасываются
export function fromHtml(html: string): RichValue {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    const { text, runs } = read(doc.body, true)
    return { text: text.replace(/\r\n?/g, '\n'), entities: entitiesOf(runs) }
}

// копирует в буфер текст и HTML (если есть разметка), с откатом на обычный writeText
export async function copyRich(value: RichValue): Promise<void> {
    const clipboard = navigator.clipboard
    if (!clipboard) {
        return
    }
    if (
        value.entities.length > 0 &&
        typeof ClipboardItem !== 'undefined' &&
        typeof clipboard.write === 'function'
    ) {
        try {
            await clipboard.write([
                new ClipboardItem({
                    'text/plain': new Blob([value.text], { type: 'text/plain' }),
                    'text/html': new Blob([toHtml(value)], { type: 'text/html' }),
                }),
            ])
            return
        } catch {
            // если браузер запретил text/html в буфере - копируем обычный текст
        }
    }
    await clipboard.writeText(value.text)
}
