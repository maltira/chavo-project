// каталог эмодзи: собран скриптом scripts/build-emoji.mjs из emojibase-data (русские названия),
// грузится при первом открытии панели, а не вместе с приложением
export type EmojiItem = [emoji: string, label: string, tags: string]

export interface EmojiGroup {
    id: GroupId
    items: EmojiItem[]
}

export type GroupId =
    'people' | 'nature' | 'food' | 'activities' | 'travel' | 'objects' | 'symbols' | 'flags'

// заголовки секций и иконки вкладок
export const GROUP_TITLES: Record<GroupId, string> = {
    people: 'Смайлы и люди',
    nature: 'Животные и природа',
    food: 'Еда и напитки',
    activities: 'Активности',
    travel: 'Путешествия',
    objects: 'Предметы',
    symbols: 'Символы',
    flags: 'Флаги',
}

export const GROUP_ICONS: Record<GroupId, string> = {
    people: 'emoji',
    nature: 'emoji-nature',
    food: 'emoji-food',
    activities: 'emoji-activities',
    travel: 'emoji-travel',
    objects: 'emoji-objects',
    symbols: 'heart',
    flags: 'emoji-flags',
}

let loading: Promise<EmojiGroup[]> | null = null

export function loadCatalog(): Promise<EmojiGroup[]> {
    loading ??= import('./emoji.ru.json').then((module) => module.default as EmojiGroup[])
    return loading
}

// поиск по названию и ключевым словам: все слова запроса должны встретиться
export function searchCatalog(groups: EmojiGroup[], query: string): EmojiItem[] {
    const words = query.toLowerCase().split(/\s+/).filter(Boolean)
    if (words.length === 0) {
        return []
    }
    return groups.flatMap((group) =>
        group.items.filter(([, label, tags]) => {
            const haystack = `${label.toLowerCase()} ${tags}`
            return words.every((word) => haystack.includes(word))
        }),
    )
}
