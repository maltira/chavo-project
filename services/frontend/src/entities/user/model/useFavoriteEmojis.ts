import { useMutation, useQueryClient } from '@tanstack/react-query'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'

import { MAX_FAVORITE_EMOJIS, updateSettings } from '../api/user'
import type { Settings } from '../api/user'
import { settingsKey, useSettings } from './useMyData'

// избранные эмодзи: лежат в настройках на сервере, поэтому одинаковы на всех устройствах;
// список меняется сразу, а при ошибке сервера возвращается прежний. enabled - настройки грузятся,
// только когда панель эмодзи открыта
export function useFavoriteEmojis(enabled = true) {
    const queryClient = useQueryClient()
    const settings = useSettings(enabled)
    const list = settings.data?.favorite_emojis ?? []

    const save = useMutation({
        mutationFn: (next: string[]) => updateSettings({ favorite_emojis: next }),
        onMutate: async (next) => {
            await queryClient.cancelQueries({ queryKey: settingsKey })
            const before = queryClient.getQueryData<Settings>(settingsKey)
            queryClient.setQueryData<Settings>(settingsKey, (s) =>
                s ? { ...s, favorite_emojis: next } : s,
            )
            return { before }
        },
        onError: (error, _next, context) => {
            queryClient.setQueryData(settingsKey, context?.before)
            notify({
                icon: 'warn',
                title: 'Не удалось сохранить избранное',
                description: errorMessage(error),
            })
        },
        onSuccess: (saved) => queryClient.setQueryData(settingsKey, saved),
    })

    function toggle(emoji: string) {
        if (list.includes(emoji)) {
            save.mutate(list.filter((e) => e !== emoji))
        } else if (list.length >= MAX_FAVORITE_EMOJIS) {
            notify({
                id: 'emoji-favorites-limit',
                icon: 'warn',
                title: `В избранном не больше ${MAX_FAVORITE_EMOJIS} эмодзи`,
            })
        } else {
            save.mutate([...list, emoji])
        }
    }

    return { list, toggle }
}
