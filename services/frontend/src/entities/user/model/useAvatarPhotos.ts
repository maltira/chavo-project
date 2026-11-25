import { useQuery } from '@tanstack/react-query'

import { getAvatarPhotos } from '../api/user'

export const avatarPhotosKey = (userId: string) => ['avatar-photos', userId] as const

// история аватарок пользователя; грузится, когда фото открыли на весь экран (enabled)
export function useAvatarPhotos(userId: string, enabled: boolean) {
    return useQuery({
        queryKey: avatarPhotosKey(userId),
        queryFn: () => getAvatarPhotos(userId),
        enabled,
    })
}
