import { useState } from 'react'
import type { MouseEvent } from 'react'

import { useBans, useGroupMembers, useJoinRequests } from '@/entities/conversation'
import type { ConversationSummary, Member } from '@/entities/conversation'
import { useMe, useProfileCard, UserRow } from '@/entities/user'
import { MemberMenu, useJoinRequestActions, useUnban } from '@/features/group/members'
import { useLoadMore } from '@/shared/lib'
import type { MenuPosition } from '@/shared/ui'
import { Button, IconButton, Segmented, SkeletonRow, TextLink } from '@/shared/ui'

import styles from './GroupInfo.module.scss'

type Tab = 'members' | 'requests' | 'bans'

interface Props {
    chat: ConversationSummary
    onAdd: () => void
}

// участники группы; у админа ещё вкладки заявок (только закрытая группа) и заблокированных
export function MembersScreen({ chat, onAdd }: Props) {
    const isAdmin = chat.my_role === 'admin'
    const withRequests = isAdmin && chat.visibility === 'private'
    const [tab, setTab] = useState<Tab>('members')
    // заявки грузятся сразу: их число - в названии вкладки
    const requests = useJoinRequests(chat.id, withRequests)
    const requestsCount = requests.data
        ? `${requests.data.length}${requests.hasNextPage ? '+' : ''}`
        : ''

    const tabs = [
        { value: 'members' as const, label: 'Участники' },
        ...(withRequests
            ? [
                  {
                      value: 'requests' as const,
                      label: requestsCount ? `Заявки (${requestsCount})` : 'Заявки',
                  },
              ]
            : []),
        ...(isAdmin ? [{ value: 'bans' as const, label: 'Заблокированы' }] : []),
    ]

    return (
        <div className={styles.members}>
            {tabs.length > 1 && (
                <Segmented aria-label="Раздел" value={tab} onChange={setTab} options={tabs} />
            )}
            {tab === 'members' && <MembersTab chat={chat} onAdd={onAdd} />}
            {tab === 'requests' && <RequestsTab groupId={chat.id} />}
            {tab === 'bans' && <BansTab groupId={chat.id} />}
        </div>
    )
}

const SKELETON_ROWS = 4

function Skeletons() {
    return (
        <div aria-busy="true" aria-label="Загрузка">
            {Array.from({ length: SKELETON_ROWS }, (_, i) => (
                <SkeletonRow key={i} />
            ))}
        </div>
    )
}

function MembersTab({ chat, onAdd }: Props) {
    const members = useGroupMembers(chat.id)
    const me = useMe()
    const openProfile = useProfileCard((state) => state.open)
    const isAdmin = chat.my_role === 'admin'
    // участник остаётся после закрытия меню: его подтверждение исключения живёт в MemberMenu
    const [menu, setMenu] = useState<{ member: Member; position: MenuPosition | null } | null>(null)
    const sentinel = useLoadMore(members, members.data?.length ?? 0)

    function onContextMenu(event: MouseEvent, member: Member) {
        // меню - только у админа и не над собой; иначе обычное меню браузера
        if (!isAdmin || member.user_id === me.data?.user_id) {
            return
        }
        event.preventDefault()
        setMenu({ member, position: { x: event.clientX, y: event.clientY } })
    }

    return (
        <>
            <p className={styles.label}>Все участники: {chat.members_count}</p>
            {members.isPending ? (
                <Skeletons />
            ) : (
                <div className={styles.list}>
                    {(members.data ?? []).map((member) => (
                        <UserRow
                            key={member.user_id}
                            userId={member.user_id}
                            onClick={() => openProfile(member.user_id)}
                            onContextMenu={(event) => onContextMenu(event, member)}
                            aside={
                                member.role === 'admin' && <span className={styles.tag}>Админ</span>
                            }
                        />
                    ))}
                    {members.hasNextPage && <div ref={sentinel} className={styles.sentinel} />}
                </div>
            )}
            {isAdmin && (
                <Button variant="secondary" className={styles.footerButton} onClick={onAdd}>
                    Добавить участника
                </Button>
            )}
            {menu && (
                <MemberMenu
                    groupId={chat.id}
                    member={menu.member}
                    position={menu.position}
                    onClose={() => setMenu({ member: menu.member, position: null })}
                />
            )}
        </>
    )
}

function RequestsTab({ groupId }: { groupId: string }) {
    const requests = useJoinRequests(groupId)
    const { approve, reject } = useJoinRequestActions(groupId)
    const openProfile = useProfileCard((state) => state.open)
    const sentinel = useLoadMore(requests, requests.data?.length ?? 0)

    if (requests.isPending) {
        return <Skeletons />
    }
    if (!requests.data?.length) {
        return <p className={styles.empty}>Новых заявок нет</p>
    }
    return (
        <div className={styles.list}>
            {requests.data.map((request) => (
                <UserRow
                    key={request.id}
                    userId={request.user_id}
                    onClick={() => openProfile(request.user_id)}
                    aside={
                        <>
                            <IconButton
                                icon="check"
                                size={40}
                                aria-label="Принять"
                                className={styles.rowButton}
                                disabled={approve.isPending || reject.isPending}
                                onClick={() => approve.mutate(request.id)}
                            />
                            <IconButton
                                icon="close"
                                size={40}
                                aria-label="Отклонить"
                                className={styles.rowButton}
                                disabled={approve.isPending || reject.isPending}
                                onClick={() => reject.mutate(request.id)}
                            />
                        </>
                    }
                />
            ))}
            {requests.hasNextPage && <div ref={sentinel} className={styles.sentinel} />}
        </div>
    )
}

function BansTab({ groupId }: { groupId: string }) {
    const bans = useBans(groupId)
    const unban = useUnban(groupId)
    const openProfile = useProfileCard((state) => state.open)
    const sentinel = useLoadMore(bans, bans.data?.length ?? 0)

    if (bans.isPending) {
        return <Skeletons />
    }
    if (!bans.data?.length) {
        return <p className={styles.empty}>Никто не заблокирован</p>
    }
    return (
        <div className={styles.list}>
            {bans.data.map((ban) => (
                <UserRow
                    key={ban.user_id}
                    userId={ban.user_id}
                    onClick={() => openProfile(ban.user_id)}
                    aside={
                        <TextLink
                            as="button"
                            className={styles.tag}
                            disabled={unban.isPending}
                            onClick={() => unban.mutate(ban.user_id)}
                        >
                            Разблокировать
                        </TextLink>
                    }
                />
            ))}
            {bans.hasNextPage && <div ref={sentinel} className={styles.sentinel} />}
        </div>
    )
}
