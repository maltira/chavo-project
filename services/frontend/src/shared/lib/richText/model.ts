// форматирование текста сообщения: «текст + диапазоны», как в Telegram. Диапазоны - в единицах UTF-16,
// то есть в индексах обычной строки JS; сервер хранит и проверяет их так же
export type EntityType =
    | 'bold'
    | 'italic'
    | 'underline'
    | 'strikethrough'
    | 'code'
    | 'pre'
    | 'spoiler'
    | 'blockquote'
    | 'text_link'
    | 'url'
    | 'mention'

export type ToggleableFormat =
    | 'bold'
    | 'italic'
    | 'underline'
    | 'strikethrough'
    | 'code'
    | 'pre'
    | 'spoiler'
    | 'blockquote'

export const ENTITY_TYPES: readonly EntityType[] = [
    'bold',
    'italic',
    'underline',
    'strikethrough',
    'code',
    'pre',
    'spoiler',
    'blockquote',
    'text_link',
    'url',
    'mention',
]

export interface MessageEntity {
    type: EntityType
    offset: number
    length: number
    url?: string
    user_id?: string
    language?: string
}

// текст и его разметка вместе
export interface RichValue {
    text: string
    entities: MessageEntity[]
}

const ORDER: Record<EntityType, number> = {
    bold: 0,
    italic: 1,
    underline: 2,
    strikethrough: 3,
    code: 4,
    pre: 5,
    spoiler: 6,
    blockquote: 7,
    text_link: 8,
    url: 9,
    mention: 10,
}

const DISTINCT_TYPES = new Set<EntityType>(['url', 'mention'])
const PLAIN_URL_RE = /https?:\/\/[^\s<>"]+/g

function hasControlOrSpace(raw: string): boolean {
    for (let i = 0; i < raw.length; i++) {
        const code = raw.charCodeAt(i)
        if (code <= 0x20 || code === 0x7f) {
            return true
        }
    }
    return /\s/.test(raw)
}

export function isSafeHttpUrl(raw: string): boolean {
    const trimmed = raw.trim()
    if (!trimmed || trimmed.length > 2048 || hasControlOrSpace(trimmed)) {
        return false
    }
    try {
        const parsed = new URL(trimmed)
        return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && !!parsed.host
    } catch {
        return false
    }
}

function cleanEntity(e: MessageEntity): MessageEntity {
    const out: MessageEntity = { type: e.type, offset: e.offset, length: e.length }
    if (e.type === 'text_link' && e.url) {
        out.url = e.url.trim()
    }
    if (e.type === 'mention' && e.user_id) {
        out.user_id = e.user_id.trim()
    }
    if (e.type === 'pre' && e.language) {
        out.language = e.language.trim()
    }
    return out
}

// канонический вид, как на сервере: диапазоны одного вида (с тем же url/language), которые пересекаются
// или касаются, склеены; url и mention не склеиваются; пустые отброшены; порядок - по началу, длинные раньше
export function normalizeEntities(entities: MessageEntity[]): MessageEntity[] {
    const out: MessageEntity[] = []
    const mergeGroups = new Map<string, MessageEntity[]>()
    const distinctGroups = new Map<EntityType, MessageEntity[]>()

    for (const raw of entities) {
        if (raw.length <= 0 || raw.offset < 0) {
            continue
        }
        const e = cleanEntity(raw)
        if (e.type === 'text_link' && (!e.url || !isSafeHttpUrl(e.url))) {
            continue
        }
        if (e.type === 'mention' && !e.user_id) {
            continue
        }
        if (DISTINCT_TYPES.has(e.type)) {
            const list = distinctGroups.get(e.type) ?? []
            list.push(e)
            distinctGroups.set(e.type, list)
            continue
        }
        const arg = e.type === 'text_link' ? (e.url ?? '') : e.type === 'pre' ? (e.language ?? '') : ''
        const key = `${e.type}\0${arg}`
        const list = mergeGroups.get(key) ?? []
        list.push(e)
        mergeGroups.set(key, list)
    }

    for (const list of mergeGroups.values()) {
        list.sort((a, b) => a.offset - b.offset)
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

    for (const list of distinctGroups.values()) {
        list.sort((a, b) => a.offset - b.offset || b.length - a.length)
        let last: MessageEntity | undefined
        for (const e of list) {
            if (last && e.offset < last.offset + last.length) {
                continue
            }
            last = { ...e }
            out.push(last)
        }
    }

    return out.sort(
        (a, b) => a.offset - b.offset || b.length - a.length || ORDER[a.type] - ORDER[b.type],
    )
}

// находит http/https-ссылки в тексте (без замыкающей пунктуации)
export function extractUrls(text: string): { url: string; offset: number; length: number }[] {
    const out: { url: string; offset: number; length: number }[] = []
    for (const match of text.matchAll(PLAIN_URL_RE)) {
        const raw = match[0]
        const offset = match.index ?? 0
        const cleaned = raw.replace(/[.,!?:;)'"\]>]+$/, '')
        if (cleaned && isSafeHttpUrl(cleaned)) {
            out.push({ url: cleaned, offset, length: cleaned.length })
        }
    }
    return out
}

// добавляет автоматические url-диапазоны для голых ссылок, не пересекающихся с кодом, ссылками и упоминаниями
export function withAutoLinks(text: string, entities: MessageEntity[]): MessageEntity[] {
    const found = extractUrls(text)
    if (found.length === 0) {
        return entities
    }
    const blocked = entities.filter(
        (e) =>
            e.type === 'url' ||
            e.type === 'text_link' ||
            e.type === 'code' ||
            e.type === 'pre' ||
            e.type === 'mention',
    )
    const extra: MessageEntity[] = []
    for (const u of found) {
        const end = u.offset + u.length
        const overlaps = blocked.some((b) => b.offset < end && b.offset + b.length > u.offset)
        if (!overlaps) {
            extra.push({ type: 'url', offset: u.offset, length: u.length })
        }
    }
    return extra.length > 0 ? normalizeEntities([...entities, ...extra]) : entities
}

// первая ссылка из text_link, url или текста сообщения (для превью ссылки)
export function extractFirstUrl(value: RichValue): string | null {
    for (const e of value.entities) {
        if (e.type === 'text_link' && e.url && isSafeHttpUrl(e.url)) {
            return e.url.trim()
        }
        if (e.type === 'url') {
            const slice = value.text.slice(e.offset, e.offset + e.length)
            if (isSafeHttpUrl(slice)) {
                return slice
            }
        }
    }
    const urls = extractUrls(value.text)
    return urls[0]?.url ?? null
}

// участок текста с одинаковым оформлением
export interface Segment {
    text: string
    // с какого индекса исходной строки начинается участок
    start: number
    bold: boolean
    italic: boolean
    underline: boolean
    strikethrough: boolean
    code: boolean
    pre: boolean
    preLanguage?: string
    spoiler: boolean
    blockquote: boolean
    linkUrl?: string
    mentionUserId?: string
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
        const covering = (type: EntityType) =>
            entities.find((e) => e.type === type && e.offset <= start && e.offset + e.length >= end)
        const preEnt = covering('pre')
        const textLinkEnt = covering('text_link')
        const urlEnt = covering('url')
        const mentionEnt = covering('mention')
        const rawLink =
            textLinkEnt?.url ??
            (urlEnt ? text.slice(urlEnt.offset, urlEnt.offset + urlEnt.length) : undefined)
        const linkUrl = rawLink && isSafeHttpUrl(rawLink) ? rawLink : undefined

        segments.push({
            text: text.slice(start, end),
            start,
            bold: !!covering('bold'),
            italic: !!covering('italic'),
            underline: !!covering('underline'),
            strikethrough: !!covering('strikethrough'),
            code: !!covering('code'),
            pre: !!preEnt,
            ...(preEnt?.language ? { preLanguage: preEnt.language } : {}),
            spoiler: !!covering('spoiler'),
            blockquote: !!covering('blockquote'),
            ...(linkUrl ? { linkUrl } : {}),
            ...(mentionEnt?.user_id ? { mentionUserId: mentionEnt.user_id } : {}),
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
    type: ToggleableFormat,
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
            rest.push({ ...e, offset: from, length: start - from })
        }
        if (to > end) {
            rest.push({ ...e, offset: end, length: to - end })
        }
    }
    return { text: value.text, entities: normalizeEntities(rest) }
}

// ищет ссылку text_link, пересекающую диапазон выделения
export function findLinkInRange(
    entities: MessageEntity[],
    start: number,
    end: number,
): string | null {
    if (end <= start) {
        return null
    }
    const found = entities.find(
        (e) => e.type === 'text_link' && e.url && e.offset < end && e.offset + e.length > start,
    )
    return found?.url ?? null
}

// назначает или снимает (url === null) ссылку text_link на диапазоне [start, end)
export function setLink(
    value: RichValue,
    start: number,
    end: number,
    url: string | null,
): RichValue {
    if (end <= start) {
        return value
    }
    const rest: MessageEntity[] = []
    for (const e of value.entities) {
        if (e.type !== 'text_link') {
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
            rest.push({ ...e, offset: from, length: start - from })
        }
        if (to > end) {
            rest.push({ ...e, offset: end, length: to - end })
        }
    }
    if (url !== null && isSafeHttpUrl(url)) {
        rest.push({ type: 'text_link', offset: start, length: end - start, url: url.trim() })
    }
    return { text: value.text, entities: normalizeEntities(rest) }
}

// поддерживаемые языки для блоков кода (pre)
export const CODE_LANGUAGES: ReadonlyArray<{ value: string; label: string }> = [
    { value: '', label: 'Без языка' },
    { value: 'go', label: 'Go' },
    { value: 'typescript', label: 'TypeScript' },
    { value: 'javascript', label: 'JavaScript' },
    { value: 'python', label: 'Python' },
    { value: 'rust', label: 'Rust' },
    { value: 'java', label: 'Java' },
    { value: 'cpp', label: 'C++' },
    { value: 'csharp', label: 'C#' },
    { value: 'sql', label: 'SQL' },
    { value: 'bash', label: 'Bash' },
    { value: 'json', label: 'JSON' },
    { value: 'yaml', label: 'YAML' },
    { value: 'html', label: 'HTML' },
    { value: 'css', label: 'CSS' },
    { value: 'dockerfile', label: 'Dockerfile' },
]

export function codeLanguageLabel(language?: string): string {
    if (!language) {
        return 'Код'
    }
    const lower = language.toLowerCase()
    const match = CODE_LANGUAGES.find((item) => item.value === lower)
    return match ? match.label : language
}

// ищет блок кода pre, пересекающий диапазон выделения ('' - без языка, null - блока нет)
export function findPreLanguageInRange(
    entities: MessageEntity[],
    start: number,
    end: number,
): string | null {
    if (end <= start) {
        return null
    }
    const found = entities.find(
        (e) => e.type === 'pre' && e.offset < end && e.offset + e.length > start,
    )
    if (!found) {
        return null
    }
    return found.language ?? ''
}

// назначает или снимает (language === null) блок кода pre на диапазоне [start, end)
export function setPre(
    value: RichValue,
    start: number,
    end: number,
    language: string | null,
): RichValue {
    if (end <= start) {
        return value
    }
    const rest: MessageEntity[] = []
    for (const e of value.entities) {
        if (e.type !== 'pre') {
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
            rest.push({ ...e, offset: from, length: start - from })
        }
        if (to > end) {
            rest.push({ ...e, offset: end, length: to - end })
        }
    }
    if (language !== null) {
        const cleanLang = language.trim().toLowerCase()
        rest.push({
            type: 'pre',
            offset: start,
            length: end - start,
            ...(cleanLang ? { language: cleanLang } : {}),
        })
    }
    return { text: value.text, entities: normalizeEntities(rest) }
}

// заменяет диапазон [start, end) упоминанием label + пробел с entity mention
export function replaceRangeWithMention(
    value: RichValue,
    start: number,
    end: number,
    label: string,
    userId: string,
): { value: RichValue; caret: number } {
    const insert = `${label} `
    const delta = insert.length - (end - start)
    const entities: MessageEntity[] = []
    for (const e of value.entities) {
        const from = e.offset
        const to = e.offset + e.length
        if (to <= start) {
            entities.push(e)
        } else if (from >= end) {
            entities.push({ ...e, offset: from + delta })
        } else if (e.type !== 'mention') {
            if (from < start) {
                entities.push({ ...e, offset: from, length: start - from })
            }
            if (to > end) {
                entities.push({ ...e, offset: start + insert.length, length: to - end })
            }
        }
    }
    entities.push({ type: 'mention', offset: start, length: label.length, user_id: userId })
    return {
        value: {
            text: value.text.slice(0, start) + insert + value.text.slice(end),
            entities: normalizeEntities(entities),
        },
        caret: start + insert.length,
    }
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
            entities.push({ ...e, offset: from, length: to - from })
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
