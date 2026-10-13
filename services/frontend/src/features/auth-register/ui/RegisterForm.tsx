import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'

import { errorMessage } from '@/shared/api'
import { notify, PASSWORD_HINT } from '@/shared/lib'
import { Button, Field, Input, Logo, PasswordInput, TextLink } from '@/shared/ui'

import { register as registerAccount } from '../api/register'
import { registerSchema } from '../model/schema'
import type { RegisterValues } from '../model/schema'
import styles from './RegisterForm.module.scss'

const ERRORS: Record<string, string> = {
    EMAIL_EXISTS: 'Эта почта уже зарегистрирована',
}

interface Props {
    // письмо ушло
    onSuccess: (email: string) => void
}

export function RegisterForm({ onSuccess }: Props) {
    const {
        register,
        handleSubmit,
        control,
        formState: { errors },
    } = useForm<RegisterValues>({
        resolver: zodResolver(registerSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
    })
    const [email, password, confirm] = useWatch({ control, name: ['email', 'password', 'confirm'] })

    const mutation = useMutation({
        mutationFn: registerAccount,
        onError: (error) =>
            notify({
                id: 'register-error',
                icon: 'warn',
                title: 'Ошибка регистрации',
                description: errorMessage(error, ERRORS),
            }),
        onSuccess: (_, values) => onSuccess(values.email),
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit(({ email, password }) => mutation.mutate({ email, password }))}
        >
            <header className={styles.header}>
                <Logo width={169} aria-label="Chavo" />
                <p className={styles.caption}>
                    Никто не слушает, пока ты не начнёшь, а когда начнёшь — никто не прервёт.
                </p>
            </header>

            <div className={styles.fields}>
                <Field label="Электронная почта" error={errors.email?.message}>
                    {(props) => (
                        <Input
                            {...props}
                            type="email"
                            autoComplete="email"
                            placeholder="example@gmail.com"
                            {...register('email')}
                        />
                    )}
                </Field>

                <Field
                    label="Придумайте пароль"
                    hint={PASSWORD_HINT}
                    error={errors.password?.message}
                >
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="new-password"
                            placeholder="Введите пароль"
                            {...register('password')}
                        />
                    )}
                </Field>

                <Field label="Повторите пароль" error={errors.confirm?.message}>
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="new-password"
                            placeholder="Введите пароль"
                            {...register('confirm')}
                        />
                    )}
                </Field>
            </div>

            <Button
                type="submit"
                className={styles.submit}
                disabled={!email || !password || !confirm || mutation.isPending}
            >
                Присоединиться
            </Button>

            <p className={styles.terms}>
                Продолжая, вы соглашаетесь с{' '}
                <TextLink className={styles.termsLink}>условиями использования</TextLink> и{' '}
                <TextLink className={styles.termsLink}>политикой конфиденциальности</TextLink>
            </p>
        </form>
    )
}
