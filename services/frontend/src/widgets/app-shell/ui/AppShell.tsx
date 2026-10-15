import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useLocation } from 'react-router'

import { cn } from '@/shared/lib'
import { IconButton, Input } from '@/shared/ui'

import { useSidebarWidth } from '../model/sidebarWidth'
import { ProfileMenu } from './ProfileMenu'
import { SidebarResizer } from './SidebarResizer'
import styles from './AppShell.module.scss'

export interface SearchSlot {
    query: string
    // выйти из поиска: очистить поле и вернуть список чатов
    close: () => void
}

interface Props {
    // список чатов; compact - сайдбар свёрнут до колонки аватаров
    list?: (compact: boolean) => ReactNode
    // результаты поиска вместо списка, пока поиск открыт
    search?: (slot: SearchSlot) => ReactNode
    // «+» - создать групповой чат
    onCreate?: () => void
    children?: ReactNode // выбранный чат
}

// окно приложения: сайдбар и область чата
export function AppShell({ list, search, onCreate, children }: Props) {
    const sidebarRef = useRef<HTMLElement>(null)
    const { width, compact, preview, commit } = useSidebarWidth()
    const [dragging, setDragging] = useState(false)
    const [query, setQuery] = useState('')
    // поиск открывается фокусом в поле и закрывается крестиком, Esc или выбором результата;
    // пустой поиск закрывается и при уходе фокуса за пределы сайдбара
    const [searching, setSearching] = useState(false)

    // любой переход (выбрали чат в поиске, вступили в группу, написали человеку) закрывает поиск:
    // поле очищается и теряет фокус, вместо результатов снова список чатов
    const { pathname } = useLocation()
    const [lastPath, setLastPath] = useState(pathname)
    if (pathname !== lastPath) {
        setLastPath(pathname)
        setQuery('')
        setSearching(false)
    }
    useEffect(() => {
        const active = document.activeElement
        if (active instanceof HTMLElement && sidebarRef.current?.contains(active)) {
            active.blur()
        }
    }, [pathname])

    function closeSearch() {
        setQuery('')
        setSearching(false)
        if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur()
        }
    }

    return (
        <div className={styles.shell}>
            <aside
                ref={sidebarRef}
                className={cn(styles.sidebar, dragging && styles.dragging)}
                style={{ width }}
            >
                <div className={styles.toolbar}>
                    <ProfileMenu />
                    {!compact && (
                        <>
                            <Input
                                placeholder="Поиск"
                                aria-label="Поиск"
                                className={styles.search}
                                value={query}
                                onChange={(event) => setQuery(event.target.value)}
                                onFocus={() => setSearching(true)}
                                onBlur={(event) => {
                                    const next = event.relatedTarget as Node | null
                                    if (!query && !sidebarRef.current?.contains(next)) {
                                        setSearching(false)
                                    }
                                }}
                                onKeyDown={(event) => {
                                    if (event.key === 'Escape') {
                                        closeSearch()
                                    }
                                }}
                                rightIcon={searching ? 'close' : undefined}
                                rightIconLabel="Закрыть поиск"
                                onRightIconClick={closeSearch}
                            />
                            <IconButton icon="add" aria-label="Новый чат" onClick={onCreate} />
                        </>
                    )}
                </div>

                {searching && !compact && search
                    ? search({ query, close: closeSearch })
                    : list?.(compact)}

                <SidebarResizer
                    target={sidebarRef}
                    onPreview={preview}
                    onCommit={commit}
                    onDragChange={setDragging}
                />
            </aside>

            <section className={styles.chat}>{children}</section>
        </div>
    )
}
