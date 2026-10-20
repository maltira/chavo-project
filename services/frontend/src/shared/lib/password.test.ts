import { newPasswordSchema } from './password'

function check(value: string) {
    const result = newPasswordSchema.safeParse(value)
    return result.success ? null : result.error.issues[0]?.message
}

describe('newPasswordSchema', () => {
    it('accepts a strong password', () => {
        expect(check('Secret#1')).toBeNull()
        expect(check('Пароль!да')).toBeNull()
    })

    it('explains the first missing requirement', () => {
        expect(check('Sec#1')).toBe('Пароль должен быть не короче 8 символов')
        expect(check('secret#1')).toBe('Добавьте заглавную букву')
        expect(check('SECRET#1')).toBe('Добавьте строчную букву')
        expect(check('Secret12')).toBe('Добавьте спецсимвол, например ! или #')
        expect(check(`Aa#${'x'.repeat(70)}`)).toBe('Пароль должен быть не длиннее 72 символов')
    })
})
