import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'

import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Button, Field, Input } from '@/shared/ui'

import { requestReset } from '../api/reset'
import { resetRequestSchema } from '../model/schema'
import type { ResetRequestValues } from '../model/schema'
import styles from './ResetForm.module.scss'

interface Props {
    onSuccess: (message: string) => void
}

export function ResetRequestForm({ onSuccess }: Props) {
    const {
        register,
        handleSubmit,
        control,
        formState: { errors },
    } = useForm<ResetRequestValues>({
        resolver: zodResolver(resetRequestSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
    })
    const email = useWatch({ control, name: 'email' })

    const mutation = useMutation({
        mutationFn: requestReset,
        onError: (error) =>
            notify({
                id: 'reset-error',
                icon: 'warn',
                title: 'Ошибка',
                description: errorMessage(error),
            }),
        onSuccess: (data) => onSuccess(data.message),
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit((values) => mutation.mutate(values))}
        >
            <header className={styles.header}>
                <h1 className={styles.title}>Восстановление пароля</h1>
                <p className={styles.text}>Введите email, который вы указали при регистрации</p>
            </header>

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

            <Button type="submit" className={styles.submit} disabled={!email || mutation.isPending}>
                Продолжить
            </Button>
        </form>
    )
}
