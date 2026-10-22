import { useQuery } from '@tanstack/react-query'

import { getCustomization } from '../api/customization'

// каталог меняется только с релизом сервера: держим час, как и Cache-Control ответа
export function useCustomization() {
    return useQuery({
        queryKey: ['customization'],
        queryFn: getCustomization,
        staleTime: 60 * 60_000,
    })
}

// подписи сроков статуса; неизвестный ключ показывается как есть
const DURATION_LABELS: Record<string, string> = {
    '1h': '1 ч',
    '4h': '4 ч',
    '24h': '24 ч',
    '7d': '7 дн',
    never: 'Всегда',
}

export function durationLabel(key: string): string {
    return DURATION_LABELS[key] ?? key
}

// подписи сроков «без звука» для чата
const MUTE_LABELS: Record<string, string> = {
    '1h': 'На 1 час',
    '8h': 'На 8 часов',
    '2d': 'На 2 дня',
    forever: 'Навсегда',
}

export function muteDurationLabel(key: string): string {
    return MUTE_LABELS[key] ?? key
}
