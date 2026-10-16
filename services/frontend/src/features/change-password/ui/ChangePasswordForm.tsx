import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'
import { z } from 'zod'

import { changePassword, useSession } from '@/entities/session'
import { ApiError, errorMessage } from '@/shared/api'
import { newPasswordSchema, notify, PASSWORD_HINT } from '@/shared/lib'
import { Button, Field, PasswordInput } from '@/shared/ui'

import styles from './ChangePasswordForm.module.scss'

const schema = z
    .object({
        current: z.string().min(1, 'Введите текущий пароль'),
        password: newPasswordSchema,
        confirm: z.string(),
    })
    .refine((values) => values.password === values.confirm, {
        path: ['confirm'],
        message: 'Пароли не совпадают',
    })

type Values = z.infer<typeof schema>

// смена пароля: сервер завершает все сеансы, включая этот, - после неё нужно войти заново
export function ChangePasswordForm() {
    const queryClient = useQueryClient()
    const signOut = useSession((state) => state.signOut)
    const {
        register,
        handleSubmit,
        control,
        setError,
        formState: { errors },
    } = useForm<Values>({
        resolver: zodResolver(schema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
    })
    const [current, password, confirm] = useWatch({
        control,
        name: ['current', 'password', 'confirm'],
    })

    const mutation = useMutation({
        mutationFn: (values: Values) => changePassword(values.current, values.password),
        onSuccess: () => {
            notify({
                id: 'password-changed',
                icon: 'check',
                title: 'Пароль изменён',
                description: 'Все сеансы завершены, войдите с новым паролем',
            })
            queryClient.clear()
            signOut()
        },
        onError: (error) => {
            if (error instanceof ApiError && error.reason === 'WRONG_PASSWORD') {
                setError('current', { message: 'Неверный текущий пароль' })
                return
            }
            if (error instanceof ApiError && error.reason === 'SAME_PASSWORD') {
                setError('password', { message: 'Новый пароль совпадает с текущим' })
                return
            }
            notify({
                icon: 'warn',
                title: 'Не удалось сменить пароль',
                description: errorMessage(error),
            })
        },
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit((values) => mutation.mutate(values))}
        >
            <div className={styles.fields}>
                <Field label="Текущий пароль" error={errors.current?.message}>
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="current-password"
                            {...register('current')}
                        />
                    )}
                </Field>
                <Field label="Новый пароль" hint={PASSWORD_HINT} error={errors.password?.message}>
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="new-password"
                            {...register('password')}
                        />
                    )}
                </Field>
                <Field
                    label="Повторите пароль"
                    error={errors.confirm?.message}
                    hint="После смены пароля все сеансы, включая этот, будут завершены"
                >
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="new-password"
                            {...register('confirm')}
                        />
                    )}
                </Field>
            </div>

            <Button
                type="submit"
                disabled={!current || !password || !confirm || mutation.isPending}
            >
                Изменить пароль
            </Button>
        </form>
    )
}
