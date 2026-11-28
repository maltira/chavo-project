export interface AvatarShape {
    shape: string
    color: string
}

export interface ProfileCover {
    color: string
    pattern: string
}

export interface Status {
    icon: string
    text: string
    background: string
    foreground: string
    expires_at: string | null
}

// профиль в ответах Gateway (GET /api/users/me и другие)
export interface Profile {
    user_id: string
    username: string
    display_name: string
    bio?: string
    avatar_url?: string
    avatar_shape?: AvatarShape | null
    profile_cover?: ProfileCover | null
    status?: Status | null
    last_seen_at: string | null
    online: boolean
    is_deleted?: boolean
    created_at: string
    updated_at: string
}
