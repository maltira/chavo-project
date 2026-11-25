import { request } from '@/shared/api'

// каталог оформления с сервера: иконки и сроки статусов, формы фигур за аватаром, сроки «без звука»
export interface Customization {
    status: { icons: string[]; durations: string[] }
    avatar_shapes: string[]
    mute_durations: string[]
}

export function getCustomization(): Promise<Customization> {
    return request<Customization>('/customization')
}
