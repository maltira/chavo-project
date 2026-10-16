import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'

import {
    blockedKey,
    blockStatusKey,
    unblockUser,
    useBlocked,
    UserAvatar,
    userKey,
} from '@/entities/user'
import type { BlockedUser } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify, settleAll } from '@/shared/lib'
import { Button, ConfirmModal, IconButton, SkeletonRow } from '@/shared/ui'

import styles from './BlockedScreen.module.scss'
import common from './SettingsModal.module.scss'

export function BlockedScreen() {
    const blocked = useBlocked()
    const queryClient = useQueryClient()
    const [confirmAll, setConfirmAll] = useState(false)

    const unblock = useMutation({
        mutationFn: (ids: string[]) => settleAll(ids, unblockUser),
        onSettled: (_, __, ids) => {
            void queryClient.invalidateQueries({ queryKey: blockedKey })
            // у разблокированных снова видны аватар, статус и онлайн, а писать им можно
            for (const id of ids) {
                void queryClient.invalidateQueries({ queryKey: blockStatusKey(id) })
                void queryClient.invalidateQueries({ queryKey: userKey(id) })
            }
        },
        // успех виден по списку; сообщаем только о неудачах, в том числе частичных
        onSuccess: ({ done, failed, firstError }, ids) => {
            if (failed === 0) {
                return
            }
            notify({
                icon: 'warn',
                title: done > 0 ? `Разблокировано ${done} из ${ids.length}` : 'Не получилось',
                description: errorMessage(firstError),
            })
        },
    })

    // «всех» - значит и тех, кто ещё не подгружен: сначала догружаем все страницы
    const { fetchNextPage, hasNextPage, isFetchingNextPage, isFetchNextPageError } = blocked
    async function unblockAll() {
        let pages = queryClient.getQueryData<{ pages: BlockedUser[][] }>(blockedKey)?.pages ?? []
        let more = hasNextPage
        while (more) {
            const next = await fetchNextPage({ cancelRefetch: false })
            pages = next.data?.pages ?? pages
            more = next.hasNextPage && !next.isError
        }
        unblock.mutate(pages.flat().map((user) => user.blocked_user_id))
    }

    // следующая страница - когда конец списка подошёл к видимой области
    const sentinelRef = useRef<HTMLDivElement>(null)
    const loaded = blocked.data?.length ?? 0
    useEffect(() => {
        const node = sentinelRef.current
        if (
            !node ||
            !hasNextPage ||
            isFetchingNextPage ||
            isFetchNextPageError ||
            typeof IntersectionObserver === 'undefined'
        ) {
            return
        }
        const observer = new IntersectionObserver(
            (entries) => {
                if (entries.some((entry) => entry.isIntersecting)) {
                    void fetchNextPage({ cancelRefetch: false })
                }
            },
            { rootMargin: '200px' },
        )
        observer.observe(node)
        return () => observer.disconnect()
    }, [hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, loaded])

    if (!blocked.data) {
        return (
            <div className={common.body}>
                <SkeletonRow />
                <SkeletonRow />
            </div>
        )
    }

    if (blocked.data.length === 0) {
        return (
            <div className={common.body}>
                <p className={common.empty}>Вы никого не блокировали</p>
            </div>
        )
    }

    return (
        <div className={common.body}>
            <Button
                variant="secondary"
                disabled={unblock.isPending}
                onClick={() => setConfirmAll(true)}
            >
                Разблокировать всех
            </Button>

            <div className={common.card}>
                {blocked.data.map((user) => (
                    <div key={user.blocked_user_id} className={styles.row}>
                        <UserAvatar src={user.avatar_url} size={40} />
                        <span className={styles.texts}>
                            <span className={styles.name}>{user.display_name}</span>
                            <span className={styles.username}>@{user.username}</span>
                        </span>
                        <IconButton
                            icon="close"
                            size={40}
                            aria-label={`Разблокировать ${user.display_name}`}
                            className={styles.remove}
                            disabled={unblock.isPending}
                            onClick={() => unblock.mutate([user.blocked_user_id])}
                        />
                    </div>
                ))}
                {isFetchingNextPage && <SkeletonRow />}
                {hasNextPage && <div ref={sentinelRef} className={styles.sentinel} />}
            </div>

            <ConfirmModal
                open={confirmAll}
                onClose={() => setConfirmAll(false)}
                onConfirm={() => {
                    setConfirmAll(false)
                    void unblockAll()
                }}
                confirmLabel="Разблокировать"
            >
                Все из списка снова смогут писать вам и видеть ваш профиль целиком
            </ConfirmModal>
        </div>
    )
}
