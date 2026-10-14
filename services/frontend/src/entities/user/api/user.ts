import { request } from '@/shared/api'

import type { Profile } from '../model/types'

// чужой профиль
export function getUser(userId: string): Promise<Profile> {
    return request<Profile>(`/users/${encodeURIComponent(userId)}`)
}
