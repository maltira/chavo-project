import { useMutation } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { ClipboardEvent, KeyboardEvent } from 'react'

import { useSession } from '@/entities/session'
import type { TwoFactorMethod } from '@/entities/session'
import { ApiError, errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Button, Field, Input, InputCode, TextLink } from '@/shared/ui'

import { resendOtp, verifyOtp } from '../api/otp'
import styles from './OtpForm.module.scss'

const CODE_LENGTH = 6
const RESEND_COOLDOWN = 60

// неверный, истёкший код и исчерпанные попытки пользователю не различаем: ответ один
const ERRORS: Record<string, string> = {
    INVALID_OTP: 'Неверный или недействительный код',
}

// ошибки, после которых этот challenge больше не годится: нужно войти заново
const EXPIRED = ['INVALID_CHALLENGE', 'OTP_ATTEMPTS_EXCEEDED']

interface Props {
    challengeId: string
    email: string
    method?: TwoFactorMethod
    // код истёк или попытки закончились: вернуть на ввод пароля, причину не раскрываем
    onExpired: () => void
}

export function OtpForm({ challengeId, email, method = 'email', onExpired }: Props) {
    const signIn = useSession((state) => state.signIn)
    const isTotp = method === 'totp'
    const [useBackup, setUseBackup] = useState(false)
    const [backupCode, setBackupCode] = useState('')
    const [digits, setDigits] = useState<string[]>(() => Array(CODE_LENGTH).fill(''))
    const [cooldown, setCooldown] = useState(isTotp ? 0 : RESEND_COOLDOWN)
    const inputs = useRef<(HTMLInputElement | null)[]>([])

    const verify = useMutation({
        mutationFn: verifyOtp,
        onSuccess: (data) => signIn(data.access_token),
        onError: (error) => {
            if (error instanceof ApiError && error.reason && EXPIRED.includes(error.reason)) {
                onExpired()
                return
            }
            notify({
                id: 'otp-error',
                icon: 'warn',
                title: 'Ошибка входа',
                description: errorMessage(error, ERRORS),
            })
        },
    })

    const resend = useMutation({
        mutationFn: resendOtp,
        onSuccess: () => {
            setCooldown(RESEND_COOLDOWN)
            setDigits(Array(CODE_LENGTH).fill(''))
            inputs.current[0]?.focus()
        },
        onError: (error) => {
            if (error instanceof ApiError && error.reason && EXPIRED.includes(error.reason)) {
                onExpired()
                return
            }
            notify({
                id: 'otp-error',
                icon: 'warn',
                title: 'Не удалось отправить код',
                description: errorMessage(error),
            })
        },
    })

    useEffect(() => {
        if (cooldown <= 0) {
            return
        }
        const timer = setTimeout(() => setCooldown((seconds) => seconds - 1), 1000)
        return () => clearTimeout(timer)
    }, [cooldown])

    const code = useBackup ? backupCode.trim() : digits.join('')
    const complete = useBackup
        ? backupCode.replace(/[\s-]/g, '').length >= 8
        : digits.join('').length === CODE_LENGTH
    const invalid = verify.isError

    function setDigit(index: number, value: string) {
        verify.reset()
        setDigits((current) => current.map((digit, i) => (i === index ? value : digit)))
        if (value && index < CODE_LENGTH - 1) {
            inputs.current[index + 1]?.focus()
        }
    }

    function onKeyDown(index: number, event: KeyboardEvent<HTMLInputElement>) {
        if (event.key === 'Backspace' && !digits[index] && index > 0) {
            event.preventDefault()
            setDigit(index - 1, '')
            inputs.current[index - 1]?.focus()
        }
    }

    // вставка целого кода раскладывается по полям
    function onPaste(event: ClipboardEvent<HTMLDivElement>) {
        const pasted = event.clipboardData.getData('text').replace(/\D/g, '').slice(0, CODE_LENGTH)
        if (!pasted) {
            return
        }
        event.preventDefault()
        verify.reset()
        setDigits(Array.from({ length: CODE_LENGTH }, (_, i) => pasted[i] ?? ''))
        inputs.current[Math.min(pasted.length, CODE_LENGTH - 1)]?.focus()
    }

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={(event) => {
                event.preventDefault()
                if (complete) {
                    verify.mutate({ challenge_id: challengeId, code })
                }
            }}
        >
            <header className={styles.header}>
                <h1 className={styles.title}>
                    {!isTotp
                        ? 'Подтверждение почты'
                        : useBackup
                          ? 'Резервный код'
                          : 'Код из приложения'}
                </h1>
                <p className={styles.text}>
                    {!isTotp ? (
                        <>
                            Код подтверждения отправлен на{' '}
                            <strong className={styles.email}>{email}</strong>
                        </>
                    ) : useBackup ? (
                        'Введите один из одноразовых резервных кодов'
                    ) : (
                        'Введите 6-значный код из приложения-аутентификатора'
                    )}
                </p>
            </header>

            {isTotp && useBackup ? (
                <Field label="Резервный код">
                    {(props) => (
                        <Input
                            {...props}
                            autoFocus
                            autoComplete="off"
                            placeholder="XXXX-XXXX"
                            aria-invalid={invalid ? true : undefined}
                            value={backupCode}
                            onChange={(event) => {
                                verify.reset()
                                setBackupCode(event.target.value.toUpperCase())
                            }}
                        />
                    )}
                </Field>
            ) : (
                <div className={styles.code} onPaste={onPaste}>
                    {digits.map((digit, index) => (
                        <InputCode
                            key={index}
                            ref={(element) => {
                                inputs.current[index] = element
                            }}
                            aria-label={`Цифра ${index + 1} из ${CODE_LENGTH}`}
                            aria-invalid={invalid ? true : undefined}
                            autoComplete={index === 0 ? 'one-time-code' : 'off'}
                            value={digit}
                            autoFocus={index === 0}
                            onChange={(event) => setDigit(index, event.target.value)}
                            onKeyDown={(event) => onKeyDown(index, event)}
                        />
                    ))}
                </div>
            )}

            <div className={styles.actions}>
                <Button
                    type="submit"
                    className={styles.submit}
                    disabled={!complete || verify.isPending}
                >
                    Продолжить
                </Button>
                {isTotp ? (
                    <p className={styles.resend}>
                        <TextLink
                            as="button"
                            onClick={() => {
                                verify.reset()
                                setUseBackup((v) => !v)
                            }}
                        >
                            {useBackup ? 'Ввести код из приложения' : 'Использовать резервный код'}
                        </TextLink>
                    </p>
                ) : (
                    <p className={styles.resend}>
                        <span className={styles.muted}>Код не пришёл?</span>{' '}
                        <TextLink
                            as="button"
                            disabled={cooldown > 0 || resend.isPending}
                            onClick={() => resend.mutate({ challenge_id: challengeId })}
                        >
                            {cooldown > 0 ? `${cooldown} с` : 'Отправить снова'}
                        </TextLink>
                    </p>
                )}
            </div>
        </form>
    )
}
