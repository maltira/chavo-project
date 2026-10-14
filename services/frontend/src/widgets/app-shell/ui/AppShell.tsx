import { useRef, useState } from 'react'
import type { ReactNode } from 'react'

import { cn } from '@/shared/lib'
import { Button, IconButton, Input } from '@/shared/ui'

import { useSidebarWidth } from '../model/sidebarWidth'
import { ProfileMenu } from './ProfileMenu'
import { SidebarResizer } from './SidebarResizer'
import styles from './AppShell.module.scss'

interface Props {
    list?: ReactNode // список чатов
    children?: ReactNode // выбранный чат
}

// окно приложения: сайдбар и область чата
export function AppShell({ list, children }: Props) {
    const sidebarRef = useRef<HTMLElement>(null)
    const { width, compact, preview, commit } = useSidebarWidth()
    const [dragging, setDragging] = useState(false)

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
                            />
                            <IconButton icon="add" aria-label="Новый чат" />
                        </>
                    )}
                </div>

                {list ??
                    (compact ? null : (
                        <div className={styles.empty}>
                            <p className={styles.emptyText}>Здесь пока что пусто</p>
                            <Button variant="secondary" className={styles.emptyButton}>
                                Искать друзей
                            </Button>
                        </div>
                    ))}

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
