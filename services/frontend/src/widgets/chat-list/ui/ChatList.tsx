import { useEffect, useRef } from 'react'

import { useConversations } from '@/entities/conversation'
import { useMe } from '@/entities/user'
import { ApiError, errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Button, SkeletonRow } from '@/shared/ui'

import styles from './ChatList.module.scss'
import { ChatListItem } from './ChatListItem'

interface Props {
    compact?: boolean // свёрнутый сайдбар: только аватары
}

const SKELETON_ROWS = 6

// список чатов сайдбара; новые сообщения и прочтения приходят по WebSocket и сразу видны здесь
export function ChatList({ compact = false }: Props) {
    const chats = useConversations()
    const me = useMe()
    const sentinelRef = useRef<HTMLDivElement>(null)
    const { hasNextPage, isFetchingNextPage, isFetchNextPageError, error, fetchNextPage } = chats
    const loaded = chats.data?.length ?? 0

    // следующая страница грузится, когда конец списка подошёл к видимой области;
    useEffect(() => {
        const node = sentinelRef.current
        if (
            !node ||
            !hasNextPage ||
            isFetchingNextPage ||
            // после ошибки автоповтора нет: иначе упавший запрос уйдёт в цикл, пока метка на виду
            isFetchNextPageError ||
            typeof IntersectionObserver === 'undefined'
        ) {
            return
        }
        const observer = new IntersectionObserver(
            (entries) => {
                if (entries.some((entry) => entry.isIntersecting)) {
                    // по умолчанию повторный вызов отменяет идущий запрос и шлёт его заново
                    void fetchNextPage({ cancelRefetch: false })
                }
            },
            { rootMargin: '200px' },
        )
        observer.observe(node)
        return () => observer.disconnect()
    }, [hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, loaded])

    // пропавшая сеть уже показана общим уведомлением, остальные ошибки подгрузки сообщаем отдельно
    useEffect(() => {
        if (
            !isFetchNextPageError ||
            (error instanceof ApiError && error.reason === 'NETWORK_ERROR')
        ) {
            return
        }
        notify({
            id: 'chats-load',
            icon: 'warn',
            title: 'Не удалось загрузить чаты',
            description: errorMessage(error),
        })
    }, [isFetchNextPageError, error])

    if (chats.isPending) {
        return (
            <div className={styles.list} aria-busy="true" aria-label="Загрузка чатов">
                {Array.from({ length: SKELETON_ROWS }, (_, i) => (
                    <SkeletonRow key={i} compact={compact} />
                ))}
            </div>
        )
    }

    // ошибка подгрузки следующей страницы список не заменяет: данные уже есть
    if (chats.isError && !chats.data) {
        return compact ? null : (
            <div className={styles.empty}>
                <p className={styles.emptyText}>Не удалось загрузить чаты</p>
                <Button
                    variant="secondary"
                    className={styles.emptyButton}
                    onClick={() => void chats.refetch()}
                >
                    Повторить
                </Button>
            </div>
        )
    }

    if (!chats.data) {
        return null
    }

    if (chats.data.length === 0) {
        return compact ? null : (
            <div className={styles.empty}>
                <p className={styles.emptyText}>Здесь пока что пусто</p>
                <Button variant="secondary" className={styles.emptyButton}>
                    Искать друзей
                </Button>
            </div>
        )
    }

    return (
        <nav className={styles.list} aria-label="Чаты">
            {chats.data.map((chat) => (
                <ChatListItem key={chat.id} chat={chat} myId={me.data?.user_id} compact={compact} />
            ))}
            {isFetchingNextPage && <SkeletonRow compact={compact} />}
            {isFetchNextPageError && !isFetchingNextPage && (
                <Button
                    variant="secondary"
                    className={styles.retry}
                    onClick={() => void fetchNextPage()}
                >
                    Повторить
                </Button>
            )}
            {/* метка конца списка для подгрузки */}
            {hasNextPage && <div ref={sentinelRef} className={styles.sentinel} />}
        </nav>
    )
}
