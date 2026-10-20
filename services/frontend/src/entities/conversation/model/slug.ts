import { z } from 'zod'

// код ссылки открытой группы: те же правила, что на сервере
export const SLUG_HINT = 'От 3 до 32 символов: латинские буквы, цифры и _'

export const slugSchema = z
    .string()
    .trim()
    .regex(/^[a-zA-Z0-9_]{3,32}$/, SLUG_HINT)
