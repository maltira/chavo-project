import { request } from '@/shared/api'

import type { Message } from '../model/types'

// all - закреп для всех участников, me - только для себя (бывает только в личном чате)
export type PinScope = 'all' | 'me'

export interface PinnedMessage {
    message: Message
    scope: PinScope
    pinned_by: string
    pinned_at: string
}

const pinsPath = (chatId: string) => `/conversations/${encodeURIComponent(chatId)}/pins`

// новые закрепы первыми
export async function getPins(chatId: string): Promise<PinnedMessage[]> {
    const res = await request<{ items: PinnedMessage[] }>(pinsPath(chatId))
    return res.items
}

export function pinMessage(chatId: string, messageId: string, scope: PinScope) {
    return request<PinnedMessage>(pinsPath(chatId), {
        method: 'POST',
        body: { message_id: messageId, scope },
    })
}

export function unpinMessage(chatId: string, messageId: string, scope: PinScope) {
    return request<void>(`${pinsPath(chatId)}/${encodeURIComponent(messageId)}?scope=${scope}`, {
        method: 'DELETE',
    })
}
