import { request } from '@/shared/api'

export interface CreateProfileBody {
    username: string
    display_name: string
    bio?: string
}

export function createProfile(body: CreateProfileBody) {
    return request<unknown>('/users', { method: 'POST', body })
}
