import { z } from 'zod'

// правила профиля - общие для создания и редактирования; совпадают с user-service: username 3-32 символа [a-zA-Z0-9_], имя 1-100, о себе до 255
export const profileSchema = z.object({
    display_name: z
        .string()
        .trim()
        .min(1, 'Введите имя')
        .max(100, 'Имя должно быть не длиннее 100 символов'),
    // ведущий @ - привычка пользователей, на сервер уходит без него
    username: z
        .string()
        .trim()
        .transform((value) => value.replace(/^@/, ''))
        .pipe(
            z
                .string()
                .min(3, 'Имя пользователя - от 3 символов')
                .max(32, 'Имя пользователя - до 32 символов')
                .regex(/^[a-zA-Z0-9_]+$/, 'Только латиница, цифры и _'),
        ),
    bio: z.string().trim().max(255, 'Не больше 255 символов'),
})

export type ProfileInput = z.input<typeof profileSchema>
export type ProfileValues = z.output<typeof profileSchema>
