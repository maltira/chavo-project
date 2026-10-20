import { useMutation, useQueryClient } from '@tanstack/react-query'

import { conversationKey, regenerateInvite } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { errorMessage } from '@/shared/api'
import { chatLinkUrl } from '@/shared/config'
import { notify } from '@/shared/lib'

// код ссылки на чат: у открытой группы - её slug, у закрытой - сгенерированный (виден только админам)
export function inviteCodeOf(chat: ConversationSummary): string | null {
    return chat.visibility === 'public' ? chat.slug : (chat.invite_code ?? null)
}

export async function copyChatLink(code: string) {
    try {
        await navigator.clipboard.writeText(chatLinkUrl(code))
        notify({ icon: 'check', title: 'Ссылка скопирована' })
    } catch {
        notify({ icon: 'warn', title: 'Не удалось скопировать ссылку' })
    }
}

// новый код закрытой группы: прежняя ссылка сразу перестаёт работать
export function useRegenerateInvite(id: string) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: () => regenerateInvite(id),
        onSuccess: (code) => {
            queryClient.setQueryData<ConversationSummary>(conversationKey(id), (chat) =>
                chat ? { ...chat, invite_code: code } : chat,
            )
            notify({
                icon: 'check',
                title: 'Ссылка обновлена',
                description: 'Прежняя больше не работает',
            })
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })
}
