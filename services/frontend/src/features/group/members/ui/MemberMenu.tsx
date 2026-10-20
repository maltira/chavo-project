import { useState } from 'react'

import type { Member } from '@/entities/conversation'
import { useProfileCard, useUser } from '@/entities/user'
import { ConfirmModal, ContextMenu, MenuItem } from '@/shared/ui'
import type { MenuPosition } from '@/shared/ui'

import { useMemberActions } from '../model/useMemberActions'

interface Props {
    groupId: string
    member: Member
    position: MenuPosition | null
    onClose: () => void
}

type Confirm = 'kick' | 'ban' | null

// действия админа над участником (правая кнопка по строке): профиль, роль, исключение, бан
export function MemberMenu({ groupId, member, position, onClose }: Props) {
    const openProfile = useProfileCard((state) => state.open)
    const user = useUser(member.user_id)
    const { role, remove, pending } = useMemberActions(groupId)
    const [confirm, setConfirm] = useState<Confirm>(null)
    const name = user.data?.display_name ?? 'Участник'
    const isAdmin = member.role === 'admin'

    return (
        <>
            <ContextMenu position={position} onClose={onClose} aria-label={`Действия: ${name}`}>
                <MenuItem icon="user" onClick={() => openProfile(member.user_id)}>
                    Профиль
                </MenuItem>
                <MenuItem
                    icon="protect"
                    onClick={() =>
                        role.mutate({ userId: member.user_id, role: isAdmin ? 'member' : 'admin' })
                    }
                >
                    {isAdmin ? 'Снять админа' : 'Сделать админом'}
                </MenuItem>
                <MenuItem icon="exit" danger onClick={() => setConfirm('kick')}>
                    Исключить
                </MenuItem>
                <MenuItem icon="block" danger onClick={() => setConfirm('ban')}>
                    Исключить и заблокировать
                </MenuItem>
            </ContextMenu>

            <ConfirmModal
                open={confirm !== null}
                onClose={() => setConfirm(null)}
                onConfirm={() => {
                    remove.mutate({ userId: member.user_id, ban: confirm === 'ban' })
                    setConfirm(null)
                }}
                confirmLabel={confirm === 'ban' ? 'Заблокировать' : 'Исключить'}
                danger
                pending={pending}
            >
                {confirm === 'ban'
                    ? `${name} покинет группу и не сможет вернуться, пока его не разблокируют`
                    : `${name} покинет группу, но сможет вернуться`}
            </ConfirmModal>
        </>
    )
}
