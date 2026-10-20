import { z } from 'zod'

import { newPasswordSchema } from '@/shared/lib'

export const registerSchema = z
    .object({
        email: z.email('Введите корректную почту'),
        password: newPasswordSchema,
        confirm: z.string(),
    })
    .refine((values) => values.password === values.confirm, {
        path: ['confirm'],
        message: 'Пароли не совпадают',
    })

export type RegisterValues = z.infer<typeof registerSchema>
