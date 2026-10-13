import { create } from 'zustand'

export interface Notice {
    id: string
    title: string
    description?: string
    icon?: string
    // без таймера: убирается только вызовом dismiss или крестиком
    persistent?: boolean
}

type NoticeInput = Omit<Notice, 'id'> & { id?: string }

const AUTO_DISMISS_MS = 5000
const MAX_VISIBLE = 3

interface NotificationsState {
    items: Notice[]
    notify: (input: NoticeInput) => string
    dismiss: (id: string) => void
}

const timers = new Map<string, ReturnType<typeof setTimeout>>()
let counter = 0

function clearTimer(id: string) {
    const timer = timers.get(id)
    if (timer) {
        clearTimeout(timer)
        timers.delete(id)
    }
}

export const useNotifications = create<NotificationsState>()((set, get) => ({
    items: [],

    // одинаковый id заменяет уже показанное уведомление и перезапускает таймер, дублей не бывает
    notify: (input) => {
        const id = input.id ?? `notice-${++counter}`
        const notice: Notice = { ...input, id }

        clearTimer(id)
        set((state) => {
            const rest = state.items.filter((item) => item.id !== id)
            const next = [...rest, notice].slice(-MAX_VISIBLE)
            // вытесненным по лимиту таймеры больше не нужны
            rest.filter((item) => !next.includes(item)).forEach((item) => clearTimer(item.id))
            return { items: next }
        })

        if (!notice.persistent) {
            timers.set(
                id,
                setTimeout(() => get().dismiss(id), AUTO_DISMISS_MS),
            )
        }
        return id
    },

    dismiss: (id) => {
        clearTimer(id)
        set((state) => ({ items: state.items.filter((item) => item.id !== id) }))
    },
}))

// вызывать можно вне компонентов: из обработчиков ошибок, подписок
export const notify = (input: NoticeInput) => useNotifications.getState().notify(input)
export const dismiss = (id: string) => useNotifications.getState().dismiss(id)
