import { z } from 'zod'

import { newPasswordSchema } from '@/shared/lib'

export const resetRequestSchema = z.object({
    email: z.email('Введите корректную почту'),
})

export const resetConfirmSchema = z
    .object({
        password: newPasswordSchema,
        confirm: z.string(),
    })
    .refine((values) => values.password === values.confirm, {
        path: ['confirm'],
        message: 'Пароли не совпадают',
    })

export type ResetRequestValues = z.infer<typeof resetRequestSchema>
export type ResetConfirmValues = z.infer<typeof resetConfirmSchema>
