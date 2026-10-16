import { request } from '@/shared/api'

// каталог оформления с сервера: иконки и сроки статусов, формы фигур за аватаром
export interface Customization {
    status: { icons: string[]; durations: string[] }
    avatar_shapes: string[]
}

export function getCustomization(): Promise<Customization> {
    return request<Customization>('/customization')
}
