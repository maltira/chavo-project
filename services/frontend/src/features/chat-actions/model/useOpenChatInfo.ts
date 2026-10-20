import { useGroupCard } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { useProfileCard } from '@/entities/user'

// «Сведения о чате»: у личного - профиль собеседника, у группы - окно группы
export function useOpenChatInfo() {
    const openProfile = useProfileCard((state) => state.open)
    const openGroup = useGroupCard((state) => state.openChat)

    return (chat: ConversationSummary) => {
        if (chat.conversation_type === 'group') {
            openGroup(chat.id)
        } else if (chat.peer_id) {
            openProfile(chat.peer_id)
        }
    }
}
