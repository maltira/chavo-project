// профиль в ответах Gateway (GET /api/users/me и другие)
export interface Profile {
    user_id: string
    username: string
    display_name: string
    bio?: string
    avatar_url?: string
    last_seen_at: string | null
    online: boolean
    created_at: string
    updated_at: string
}
