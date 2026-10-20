import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'
import { Link } from 'react-router'

import { errorMessage } from '@/shared/api'
import { ROUTES } from '@/shared/config'
import { notify } from '@/shared/lib'
import { Button, Field, Input, Logo, PasswordInput, TextLink } from '@/shared/ui'

import { login } from '../api/login'
import { loginSchema } from '../model/schema'
import type { LoginValues } from '../model/schema'
import styles from './LoginForm.module.scss'

const ERRORS: Record<string, string> = {
    INVALID_CREDENTIALS: 'Неверная почта или пароль',
    ACCOUNT_NOT_VERIFIED: 'Подтвердите почту по ссылке из письма',
}

interface Props {
    onSuccess: (challenge: { challengeId: string; email: string }) => void
}

export function LoginForm({ onSuccess }: Props) {
    // ошибки поля появляются только после попытки отправки, дальше исчезают по мере исправления
    const {
        register,
        handleSubmit,
        control,
        formState: { errors },
    } = useForm<LoginValues>({
        resolver: zodResolver(loginSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
    })
    const email = useWatch({ control, name: 'email' })
    const password = useWatch({ control, name: 'password' })

    const mutation = useMutation({
        mutationFn: login,
        onError: (error) =>
            notify({
                id: 'login-error',
                icon: 'warn',
                title: 'Ошибка входа',
                description: errorMessage(error, ERRORS),
            }),
        onSuccess: (data, values) =>
            onSuccess({ challengeId: data.challenge_id, email: values.email }),
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit((values) => mutation.mutate(values))}
        >
            <header className={styles.header}>
                <Logo width={169} aria-label="Chavo" />
                <p className={styles.caption}>Только твой голос и те, кто действительно важен.</p>
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

                <div className={styles.password}>
                    <Field label="Пароль" error={errors.password?.message}>
                        {(props) => (
                            <PasswordInput
                                {...props}
                                autoComplete="current-password"
                                placeholder="Введите пароль"
                                {...register('password')}
                            />
                        )}
                    </Field>
                    <TextLink as={Link} to={ROUTES.reset} className={styles.forgot}>
                        Забыли пароль?
                    </TextLink>
                </div>
            </div>

            <Button
                type="submit"
                className={styles.submit}
                disabled={!email || !password || mutation.isPending}
            >
                Войти
            </Button>

            <p className={styles.footer}>
                <span className={styles.muted}>Нет аккаунта?</span>{' '}
                <TextLink as={Link} to={ROUTES.register}>
                    Регистрация
                </TextLink>
            </p>
        </form>
    )
}
