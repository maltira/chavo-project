import { useGroupCard } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { useProfileCard } from '@/entities/user'

// «Сведения о чате»: у личного - профиль собеседника, у группы - карточка группы
export function useOpenChatInfo() {
    const openProfile = useProfileCard((state) => state.open)
    const openGroup = useGroupCard((state) => state.open)

    return (chat: ConversationSummary) => {
        if (chat.conversation_type === 'group') {
            openGroup({
                id: chat.id,
                name: chat.name ?? 'Группа',
                description: chat.description,
                avatar_url: chat.avatar_url,
                members_count: null,
                is_member: true,
                status: chat.status,
                avatar_shape: chat.avatar_shape,
            })
        } else if (chat.peer_id) {
            openProfile(chat.peer_id)
        }
    }
}
