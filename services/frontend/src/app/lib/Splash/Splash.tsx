import { useEffect, useState } from 'react'

import { Loader } from '@/shared/ui'

import styles from './Splash.module.scss'

const STAGES = [
    { after: 0, text: 'Подготавливаем ваши чаты' },
    { after: 6_000, text: 'Сегодня чуть медленнее, чем обычно' },
    { after: 15_000, text: 'Всё ещё загружаем, проверьте соединение' },
    { after: 30_000, text: 'Что-то пошло не так, попробуйте обновить страницу' },
]

const LAST_STAGE = STAGES.length - 1

// причина, по которой ждём: вместо текстов по времени - объяснение, что повторим сами
const UNREACHABLE_TEXT = {
    offline: 'Нет подключения к интернету. Подключимся, как только оно появится',
    server: 'Сервер не отвечает. Пробуем подключиться снова',
} as const

// отсчёт от запуска приложения
const APP_START = Date.now()

function stageAt(elapsed: number) {
    let stage = 0
    STAGES.forEach((item, index) => {
        if (elapsed >= item.after) {
            stage = index
        }
    })
    return stage
}

interface Props {
    // запрос уже повторяется: сразу переходим к тексту про медленную загрузку
    retrying?: boolean
    // загрузка окончательно упала: сразу последний текст с просьбой обновить страницу
    failed?: boolean
    // точка отсчёта, мс; по умолчанию запуск приложения
    since?: number
    // связи нет: показываем причину, приложение повторяет попытку само
    unreachable?: keyof typeof UNREACHABLE_TEXT | null
}

// экран ожидания: лоадер и пояснение, которое меняется по мере ожидания
export function Splash({
    retrying = false,
    failed = false,
    since = APP_START,
    unreachable = null,
}: Props) {
    const [now, setNow] = useState(() => Date.now())

    useEffect(() => {
        if (failed) {
            return
        }
        const timer = setInterval(() => setNow(Date.now()), 1000)
        return () => clearInterval(timer)
    }, [failed])

    let stage = stageAt(now - since)
    if (retrying) {
        stage = Math.max(stage, 1)
    }
    if (failed) {
        stage = LAST_STAGE
    }

    return (
        <div className={styles.root}>
            <Loader />
            <p className={styles.text} aria-live="polite" data-testid="splash-text">
                {unreachable ? UNREACHABLE_TEXT[unreachable] : STAGES[stage]?.text}
            </p>
        </div>
    )
}
