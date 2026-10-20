import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
    addMembers,
    approveJoinRequest,
    invalidateGroup,
    rejectJoinRequest,
    removeMember,
    setMemberRole,
    unban,
} from '@/entities/conversation'
import type { Member } from '@/entities/conversation'
import { errorMessage } from '@/shared/api'
import { notify, settleAll } from '@/shared/lib'

const failed = (title: string) => (error: unknown) =>
    notify({ icon: 'warn', title, description: errorMessage(error) })

// роль, исключение и бан участника; после любого - перечитать группу (придёт и событие)
export function useMemberActions(groupId: string) {
    const queryClient = useQueryClient()
    const onSettled = () => invalidateGroup(queryClient, groupId)

    const role = useMutation({
        mutationFn: ({ userId, role }: { userId: string; role: Member['role'] }) =>
            setMemberRole(groupId, userId, role),
        onSettled,
        // LAST_ADMIN: последнего админа сервер не разжалует - объяснит текстом ошибки
        onError: failed('Не удалось изменить роль'),
    })
    const remove = useMutation({
        mutationFn: ({ userId, ban }: { userId: string; ban: boolean }) =>
            removeMember(groupId, userId, ban),
        onSettled,
        onError: failed('Не удалось исключить'),
    })

    return { role, remove, pending: role.isPending || remove.isPending }
}

// добавление нескольких: каждый по отдельности, чтобы один запретивший приглашения не сорвал остальных
export function useAddMembers(groupId: string, onDone: () => void) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (userIds: string[]) => settleAll(userIds, (id) => addMembers(groupId, [id])),
        onSettled: () => invalidateGroup(queryClient, groupId),
        onSuccess: ({ done, failed: count, firstError }, ids) => {
            if (count === 0) {
                notify({
                    icon: 'check',
                    title: done === 1 ? 'Участник добавлен' : 'Участники добавлены',
                })
                onDone()
                return
            }
            notify({
                icon: 'warn',
                title: done > 0 ? `Добавлено ${done} из ${ids.length}` : 'Не удалось добавить',
                description: errorMessage(firstError),
            })
            if (done > 0) {
                onDone()
            }
        },
    })
}

// решение по заявке в закрытую группу
export function useJoinRequestActions(groupId: string) {
    const queryClient = useQueryClient()
    const onSettled = () => invalidateGroup(queryClient, groupId)
    const approve = useMutation({
        mutationFn: (requestId: string) => approveJoinRequest(groupId, requestId),
        onSettled,
        onError: failed('Не удалось принять заявку'),
    })
    const reject = useMutation({
        mutationFn: (requestId: string) => rejectJoinRequest(groupId, requestId),
        onSettled,
        onError: failed('Не удалось отклонить заявку'),
    })
    return { approve, reject }
}

export function useUnban(groupId: string) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (userId: string) => unban(groupId, userId),
        onSettled: () => invalidateGroup(queryClient, groupId),
        onError: failed('Не удалось разблокировать'),
    })
}
