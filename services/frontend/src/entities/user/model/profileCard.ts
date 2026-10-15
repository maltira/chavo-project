import { create } from 'zustand'

import { useRecentUsers } from './recent'

interface ProfileCardState {
    // чей профиль открыт в модалке; null - закрыта
    userId: string | null
    open: (userId: string, options?: { remember?: boolean }) => void
    close: () => void
}

// модалка профиля открывается из поиска, шапки чата и меню профиля, а показывается в одном месте;
// открытый чужой профиль попадает в «Недавние»
export const useProfileCard = create<ProfileCardState>()((set) => ({
    userId: null,
    open: (userId, { remember = true } = {}) => {
        if (remember) {
            useRecentUsers.getState().add(userId)
        }
        set({ userId })
    },
    close: () => set({ userId: null }),
}))
