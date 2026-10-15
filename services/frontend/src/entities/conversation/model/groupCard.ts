import { create } from 'zustand'

import type { PublicGroup } from '../api/conversations'

interface GroupCardState {
    // открытая в модалке группа; null - закрыта
    group: PublicGroup | null
    open: (group: PublicGroup) => void
    close: () => void
}

// модалка группы из поиска: открывается в одном месте, показывается на странице
export const useGroupCard = create<GroupCardState>()((set) => ({
    group: null,
    open: (group) => set({ group }),
    close: () => set({ group: null }),
}))
