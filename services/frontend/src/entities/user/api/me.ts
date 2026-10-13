import { ApiError, request } from '@/shared/api'

import type { Profile } from '../model/types'

// null - профиль ещё не создан (сервер отвечает 404)
export async function getMe(): Promise<Profile | null> {
    try {
        return await request<Profile>('/users/me')
    } catch (error) {
        if (error instanceof ApiError && error.status === 404) { // профиль не создан
            return null
        }
        throw error
    }
}
