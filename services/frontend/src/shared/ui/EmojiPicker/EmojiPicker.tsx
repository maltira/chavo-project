import { useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'

import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import { GROUP_ICONS, GROUP_TITLES, loadCatalog, searchCatalog } from './catalog'
import type { EmojiGroup, EmojiItem, GroupId } from './catalog'
import styles from './EmojiPicker.module.scss'
import { getFrequent, recordEmoji } from './frequent'

interface Props {
    // выбранный эмодзи вставляется в поле; панель остаётся открытой - можно выбрать несколько
    // keyboard - выбор с клавиатуры (Enter, пробел): тогда фокус должен остаться в панели
    onPick: (emoji: string, options?: { keyboard: boolean }) => void
    // избранные (из настроек) и их изменение: правая кнопка мыши или клавиша меню на эмодзи
    favorites: string[]
    onToggleFavorite: (emoji: string) => void
}

// секции списка: у «Избранных» и «Часто используемых» свои заголовки и иконки вкладок
type SectionId = 'favorites' | 'frequent' | GroupId

const TAB_ICONS: Record<'favorites' | 'frequent', string> = {
    favorites: 'star',
    frequent: 'emoji-recent',
}
const TITLES: Record<SectionId, string> = {
    favorites: 'Избранные',
    frequent: 'Часто используемые',
    ...GROUP_TITLES,
}

// панель эмодзи над полем ввода: поиск, вкладки категорий, «Избранные», «Часто используемые» и категории
export function EmojiPicker({ onPick, favorites, onToggleFavorite }: Props) {
    const [groups, setGroups] = useState<EmojiGroup[] | null>(null)
    const [failed, setFailed] = useState(false)
    const [query, setQuery] = useState('')
    const [active, setActive] = useState<SectionId>('people')
    // «часто используемые» читаются при открытии: выбор внутри открытой панели порядок не меняет
    const [frequent] = useState(() => getFrequent())
    const scrollerRef = useRef<HTMLDivElement>(null)
    const searchRef = useRef<HTMLInputElement>(null)
    const sectionRefs = useRef<Partial<Record<SectionId, HTMLElement | null>>>({})

    useEffect(() => {
        let alive = true
        loadCatalog().then(
            (loaded) => alive && setGroups(loaded),
            () => alive && setFailed(true),
        )
        return () => {
            alive = false
        }
    }, [])

    const labels = useMemo(() => {
        const map = new Map<string, string>()
        groups?.forEach((g) => g.items.forEach(([emoji, label]) => map.set(emoji, label)))
        return map
    }, [groups])

    const found = useMemo(
        () => (groups && query.trim() ? searchCatalog(groups, query) : null),
        [groups, query],
    )

    function registerSection(id: SectionId, el: HTMLElement | null) {
        sectionRefs.current[id] = el
    }

    function pick(emoji: string, keyboard: boolean) {
        recordEmoji(emoji)
        onPick(emoji, { keyboard })
    }

    function goTo(id: SectionId) {
        const scroller = scrollerRef.current
        const section = sectionRefs.current[id]
        if (scroller && section) {
            scroller.scrollTo({ top: section.offsetTop - scroller.offsetTop, behavior: 'smooth' })
        }
        setActive(id)
    }

    // активная вкладка - секция, которая сейчас у верхнего края списка
    function onScroll() {
        const scroller = scrollerRef.current
        if (!scroller) {
            return
        }
        const top = scroller.scrollTop + scroller.offsetTop + 8
        let current: SectionId | null = null
        for (const [id, el] of Object.entries(sectionRefs.current) as [SectionId, HTMLElement][]) {
            if (el && el.offsetTop <= top) {
                current = id
            }
        }
        if (current) {
            setActive(current)
        }
    }

    // стрелки двигают фокус по сетке: влево и вправо - по порядку, вверх и вниз - к ближайшему по горизонтали
    function onGridKeyDown(event: KeyboardEvent<HTMLElement>) {
        const target = event.target as HTMLElement
        if (!target.dataset.emoji) {
            return
        }
        const buttons = [
            ...(scrollerRef.current?.querySelectorAll<HTMLElement>('button[data-emoji]') ?? []),
        ]
        const index = buttons.indexOf(target)
        let next: HTMLElement | undefined
        if (event.key === 'ArrowRight') {
            next = buttons[index + 1]
        } else if (event.key === 'ArrowLeft') {
            next = buttons[index - 1]
        } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            const down = event.key === 'ArrowDown'
            const here = target.getBoundingClientRect()
            let best = Infinity
            for (const button of buttons) {
                const rect = button.getBoundingClientRect()
                const rowDelta = down ? rect.top - here.top : here.top - rect.top
                if (rowDelta < 1) {
                    continue
                }
                // ближайший ряд, в нём - ближайший по горизонтали
                const score = rowDelta * 1000 + Math.abs(rect.left - here.left)
                if (score < best) {
                    best = score
                    next = button
                }
            }
            if (!next && event.key === 'ArrowUp') {
                searchRef.current?.focus()
            }
        } else {
            return
        }
        event.preventDefault()
        next?.focus()
    }

    const cell = (emoji: string, key: string) => (
        <button
            key={key}
            type="button"
            className={cn(styles.emoji, favorites.includes(emoji) && styles.favorite)}
            data-emoji={emoji}
            aria-label={labels.get(emoji) ?? emoji}
            title={labels.get(emoji)}
            // detail 0 - нажатие сделано клавиатурой, а не мышью
            onClick={(event) => pick(emoji, event.detail === 0)}
            onContextMenu={(event) => {
                event.preventDefault()
                onToggleFavorite(emoji)
            }}
        >
            {emoji}
        </button>
    )

    const tabs: SectionId[] = [
        'favorites',
        ...(frequent.length > 0 ? (['frequent'] as const) : []),
        ...(groups?.map((g) => g.id) ?? []),
    ]

    return (
        <div className={styles.panel} role="dialog" aria-label="Эмодзи">
            <div className={styles.search}>
                <Icon name="search" size={20} color="inherit" />
                <input
                    ref={searchRef}
                    autoFocus
                    className={styles.searchInput}
                    type="search"
                    placeholder="Поиск эмодзи..."
                    aria-label="Поиск эмодзи"
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    onKeyDown={(event) => {
                        if (event.key === 'ArrowDown') {
                            event.preventDefault()
                            scrollerRef.current
                                ?.querySelector<HTMLElement>('button[data-emoji]')
                                ?.focus()
                        }
                    }}
                />
            </div>

            {!found && (
                <div className={styles.tabs} role="toolbar" aria-label="Категории">
                    {tabs.map((id) => (
                        <button
                            key={id}
                            type="button"
                            className={cn(styles.tab, active === id && styles.tabActive)}
                            aria-label={TITLES[id]}
                            aria-pressed={active === id}
                            title={TITLES[id]}
                            onClick={() => goTo(id)}
                        >
                            <Icon
                                name={
                                    id === 'favorites' || id === 'frequent'
                                        ? TAB_ICONS[id]
                                        : GROUP_ICONS[id]
                                }
                                size={22}
                                color="inherit"
                            />
                        </button>
                    ))}
                </div>
            )}

            <div
                ref={scrollerRef}
                className={styles.scroller}
                onScroll={found ? undefined : onScroll}
                onKeyDown={onGridKeyDown}
            >
                {failed && <p className={styles.hint}>Не удалось загрузить эмодзи</p>}
                {!groups && !failed && <p className={styles.hint}>Загрузка...</p>}

                {found && (
                    <section aria-label="Результаты поиска">
                        {found.length > 0 ? (
                            <div className={styles.grid}>
                                {found.map(([emoji]: EmojiItem) => cell(emoji, emoji))}
                            </div>
                        ) : (
                            <p className={styles.hint}>Ничего не найдено</p>
                        )}
                    </section>
                )}

                {groups && !found && (
                    <>
                        <Section id="favorites" register={registerSection}>
                            {favorites.length > 0 ? (
                                <div className={styles.grid}>
                                    {favorites.map((emoji) => cell(emoji, `fav-${emoji}`))}
                                </div>
                            ) : (
                                <p className={styles.hint}>
                                    Правая кнопка мыши по эмодзи добавляет его в избранное
                                </p>
                            )}
                        </Section>
                        {frequent.length > 0 && (
                            <Section id="frequent" register={registerSection}>
                                <div className={styles.grid}>
                                    {frequent.map((emoji) => cell(emoji, `freq-${emoji}`))}
                                </div>
                            </Section>
                        )}
                        {groups.map((group) => (
                            <Section key={group.id} id={group.id} register={registerSection}>
                                <div className={styles.grid}>
                                    {group.items.map(([emoji]) => cell(emoji, emoji))}
                                </div>
                            </Section>
                        ))}
                    </>
                )}
            </div>
        </div>
    )
}

interface SectionProps {
    id: SectionId
    // запоминает элемент секции: по нему работают переход по вкладке и подсветка активной
    register: (id: SectionId, el: HTMLElement | null) => void
    children: ReactNode
}

function Section({ id, register, children }: SectionProps) {
    return (
        <section ref={(el) => register(id, el)} className={styles.section} aria-label={TITLES[id]}>
            <h3 className={styles.title}>{TITLES[id]}</h3>
            {children}
        </section>
    )
}
