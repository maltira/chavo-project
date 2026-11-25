// форматирование текста сообщения: «текст + диапазоны», как в Telegram. Диапазоны - в единицах UTF-16,
// то есть в индексах обычной строки JS; сервер хранит и проверяет их так же
export type EntityType = 'bold' | 'italic' | 'underline'

export const ENTITY_TYPES: readonly EntityType[] = ['bold', 'italic', 'underline']

export interface MessageEntity {
    type: EntityType
    offset: number
    length: number
}

// текст и его разметка вместе
export interface RichValue {
    text: string
    entities: MessageEntity[]
}

const ORDER: Record<EntityType, number> = { bold: 0, italic: 1, underline: 2 }

// канонический вид, как на сервере: диапазоны одного вида, которые пересекаются или касаются, склеены;
// пустые отброшены; порядок - по началу, при равенстве длинные раньше
export function normalizeEntities(entities: MessageEntity[]): MessageEntity[] {
    const out: MessageEntity[] = []
    for (const type of ENTITY_TYPES) {
        const list = entities
            .filter((e) => e.type === type && e.length > 0)
            .sort((a, b) => a.offset - b.offset)
        let current: MessageEntity | undefined
        for (const e of list) {
            if (current && e.offset <= current.offset + current.length) {
                current.length =
                    Math.max(current.offset + current.length, e.offset + e.length) - current.offset
            } else {
                current = { ...e }
                out.push(current)
            }
        }
    }
    return out.sort(
        (a, b) => a.offset - b.offset || b.length - a.length || ORDER[a.type] - ORDER[b.type],
    )
}

// участок текста с одинаковым оформлением
export interface Segment {
    text: string
    // с какого индекса исходной строки начинается участок
    start: number
    bold: boolean
    italic: boolean
    underline: boolean
}

// режет текст по границам диапазонов; участки идут подряд и покрывают весь текст
export function toSegments(text: string, entities: MessageEntity[]): Segment[] {
    if (text === '') {
        return []
    }
    const cuts = new Set<number>([0, text.length])
    for (const e of entities) {
        cuts.add(Math.max(0, Math.min(text.length, e.offset)))
        cuts.add(Math.max(0, Math.min(text.length, e.offset + e.length)))
    }
    const points = [...cuts].sort((a, b) => a - b)
    const segments: Segment[] = []
    for (let i = 0; i < points.length - 1; i++) {
        const start = points[i]!
        const end = points[i + 1]!
        const covers = (type: EntityType) =>
            entities.some((e) => e.type === type && e.offset <= start && e.offset + e.length >= end)
        segments.push({
            text: text.slice(start, end),
            start,
            bold: covers('bold'),
            italic: covers('italic'),
            underline: covers('underline'),
        })
    }
    return segments
}

// весь ли диапазон [start, end) уже оформлен этим видом
export function hasFormat(
    entities: MessageEntity[],
    type: EntityType,
    start: number,
    end: number,
): boolean {
    if (end <= start) {
        return false
    }
    // склеенные диапазоны: достаточно одного, который накрывает отрезок целиком
    return normalizeEntities(entities).some(
        (e) => e.type === type && e.offset <= start && e.offset + e.length >= end,
    )
}

// включает оформление на диапазоне, а если он уже оформлен целиком - снимает
export function toggleFormat(
    value: RichValue,
    type: EntityType,
    start: number,
    end: number,
): RichValue {
    if (end <= start) {
        return value
    }
    if (!hasFormat(value.entities, type, start, end)) {
        return {
            text: value.text,
            entities: normalizeEntities([
                ...value.entities,
                { type, offset: start, length: end - start },
            ]),
        }
    }
    // вычитаем [start, end) из диапазонов этого вида
    const rest: MessageEntity[] = []
    for (const e of value.entities) {
        if (e.type !== type) {
            rest.push(e)
            continue
        }
        const from = e.offset
        const to = e.offset + e.length
        if (to <= start || from >= end) {
            rest.push(e)
            continue
        }
        if (from < start) {
            rest.push({ type, offset: from, length: start - from })
        }
        if (to > end) {
            rest.push({ type, offset: end, length: to - end })
        }
    }
    return { text: value.text, entities: normalizeEntities(rest) }
}

// убирает пробелы и переводы строк по краям; диапазоны сдвигаются и обрезаются, опустевшие пропадают
export function trimValue(value: RichValue): RichValue {
    const lead = value.text.length - value.text.trimStart().length
    const text = value.text.trim()
    const entities: MessageEntity[] = []
    for (const e of value.entities) {
        const from = Math.max(0, e.offset - lead)
        const to = Math.min(text.length, e.offset + e.length - lead)
        if (to > from) {
            entities.push({ type: e.type, offset: from, length: to - from })
        }
    }
    return { text, entities: normalizeEntities(entities) }
}

export function sameValue(a: RichValue, b: RichValue): boolean {
    return (
        a.text === b.text &&
        JSON.stringify(normalizeEntities(a.entities)) ===
            JSON.stringify(normalizeEntities(b.entities))
    )
}
