import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'

import { ApiError, errorMessage } from '@/shared/api'
import { notify, PASSWORD_HINT } from '@/shared/lib'
import { Button, Field, PasswordInput } from '@/shared/ui'

import { confirmReset } from '../api/reset'
import { resetConfirmSchema } from '../model/schema'
import type { ResetConfirmValues } from '../model/schema'
import styles from './ResetForm.module.scss'

interface Props {
    token: string
    onSuccess: () => void
    onInvalidToken: () => void // ссылка из письма устарела или уже использована
}

export function ResetConfirmForm({ token, onSuccess, onInvalidToken }: Props) {
    const {
        register,
        handleSubmit,
        control,
        formState: { errors },
    } = useForm<ResetConfirmValues>({
        resolver: zodResolver(resetConfirmSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
    })
    const [password, confirm] = useWatch({ control, name: ['password', 'confirm'] })

    const mutation = useMutation({
        mutationFn: confirmReset,
        onError: (error) => {
            if (error instanceof ApiError && error.reason === 'INVALID_TOKEN') {
                onInvalidToken()
                return
            }
            notify({
                id: 'reset-error',
                icon: 'warn',
                title: 'Ошибка',
                description: errorMessage(error, {
                    SAME_PASSWORD: 'Новый пароль совпадает со старым',
                }),
            })
        },
        onSuccess: () => onSuccess(),
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit((values) =>
                mutation.mutate({ token, password: values.password }),
            )}
        >
            <header className={styles.header}>
                <h1 className={styles.title}>Придумайте пароль</h1>
                <p className={styles.text}>Убедитесь, что не забудете его вновь</p>
            </header>

            <div className={styles.fields}>
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
                disabled={!password || !confirm || mutation.isPending}
            >
                Изменить пароль
            </Button>
        </form>
    )
}
