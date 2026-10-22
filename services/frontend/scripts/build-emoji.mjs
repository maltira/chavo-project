// собирает компактный набор эмодзи для панели из emojibase-data: только базовые эмодзи (без оттенков кожи),
// поддержанные почти везде (Unicode до 15.0), русские названия и ключевые слова для поиска.
// Запуск: npm run emoji:build; результат (src/shared/ui/EmojiPicker/emoji.ru.json) лежит в репозитории,
// поэтому обычная сборка от пакета не зависит
import { readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const data = JSON.parse(readFileSync(require.resolve('emojibase-data/ru/data.json'), 'utf8'))

// группа emojibase -> наша категория; people объединяет смайлы и людей
const CATEGORY = { 0: 'people', 1: 'people', 3: 'nature', 4: 'food', 6: 'activities', 5: 'travel', 7: 'objects', 8: 'symbols', 9: 'flags' }
const ORDER = ['people', 'nature', 'food', 'activities', 'travel', 'objects', 'symbols', 'flags']
// последний Unicode, который ещё нормально рисуют системные шрифты
const MAX_VERSION = 15.0

const groups = Object.fromEntries(ORDER.map((id) => [id, []]))
for (const e of [...data].sort((a, b) => (a.order ?? 0) - (b.order ?? 0))) {
    const id = CATEGORY[e.group]
    if (!id || !e.emoji || parseFloat(e.version) > MAX_VERSION) {
        continue
    }
    // эмодзи, название и ключевые слова (по ним и названию идёт поиск) в нижнем регистре
    groups[id].push([e.emoji, e.label, (e.tags ?? []).join(' ').toLowerCase()])
}

const out = ORDER.map((id) => ({ id, items: groups[id] }))
const total = out.reduce((n, g) => n + g.items.length, 0)
writeFileSync(new URL('../src/shared/ui/EmojiPicker/emoji.ru.json', import.meta.url), JSON.stringify(out))
console.log(`emoji: ${total} in ${out.length} groups`)
