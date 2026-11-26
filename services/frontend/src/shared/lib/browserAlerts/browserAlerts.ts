import { create } from 'zustand'

const SOUND_KEY = 'chavo:notif-sound'
const DESKTOP_KEY = 'chavo:notif-desktop'

const BASE_TITLE = 'Chavo'
const BASE_FAVICON = '/favicon.svg'
// логотип из public/favicon.svg + красная точка в правом верхнем углу при непрочитанных
const BADGED_FAVICON =
    'data:image/svg+xml,' +
    encodeURIComponent(
        '<svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">' +
            '<path d="M6.05796 3.38571C6.53405 2.90969 7.26473 2.86905 7.69077 3.29489L15.6858 11.2892C16.4124 12.0156 16.7756 12.3788 17.0843 12.251C17.3929 12.1231 17.3929 11.6094 17.3929 10.582V9.4443C17.3929 8.771 17.9393 8.22457 18.6126 8.22457C19.2858 8.22481 19.8314 8.77115 19.8314 9.4443V10.7145C19.8314 11.7417 19.8314 12.2552 20.14 12.3831C20.4486 12.511 20.8118 12.1479 21.5383 11.4218L23.9759 8.98532C24.402 8.55954 25.1336 8.60014 25.6097 9.07614C26.0859 9.5522 26.1266 10.284 25.7005 10.7099L23.394 13.0153C22.6671 13.7419 22.3036 14.1052 22.4314 14.4139C22.5593 14.7226 23.0732 14.7226 24.1009 14.7226H25.1087C25.7818 14.7229 26.3275 15.2683 26.3275 15.9414C26.3275 16.6145 25.7818 17.1599 25.1087 17.1601H23.9722C22.9447 17.1601 22.431 17.1601 22.3031 17.4688C22.1753 17.7774 22.5385 18.1407 23.2651 18.8672L25.6908 21.2929C26.1166 21.719 26.076 22.4506 25.5999 22.9267C25.1239 23.4028 24.3922 23.4433 23.9662 23.0175L21.5385 20.5899C20.812 19.8634 20.4487 19.5001 20.1401 19.6279C19.8314 19.7558 19.8314 20.2695 19.8314 21.297V22.4482C19.8311 23.1211 19.2856 23.6667 18.6126 23.667C17.9395 23.667 17.3932 23.1213 17.3929 22.4482V21.4273C17.3929 20.4001 17.3929 19.8864 17.0843 19.7586C16.7757 19.6307 16.4124 19.9938 15.6859 20.7201L7.69858 28.705C7.27249 29.131 6.54101 29.0903 6.06479 28.6142C5.58884 28.1382 5.54808 27.4073 5.97397 26.9814L14.0905 18.8673C14.8172 18.1408 15.1806 17.7775 15.0528 17.4688C14.9249 17.1601 14.4111 17.1601 13.3835 17.1601H12.1048C11.4316 17.1601 10.8861 16.6146 10.8861 15.9414C10.8861 15.2681 11.4316 14.7227 12.1048 14.7226H13.257C14.2845 14.7226 14.7982 14.7226 14.9261 14.414C15.0539 14.1053 14.6907 13.742 13.9641 13.0155L5.96714 5.01852C5.54121 4.5925 5.58184 3.8618 6.05796 3.38571Z" fill="#D97757"/>' +
            '<circle cx="25" cy="7" r="6" fill="#E5484D" stroke="#181818" stroke-width="2"/>' +
            '</svg>',
    )

function readFlag(key: string, fallback: boolean): boolean {
    try {
        const raw = localStorage.getItem(key)
        if (raw === '0') return false
        if (raw === '1') return true
    } catch {
        // без хранилища берём значение по умолчанию
    }
    return fallback
}

function writeFlag(key: string, value: boolean) {
    try {
        localStorage.setItem(key, value ? '1' : '0')
    } catch {
        // настройка останется в памяти до перезагрузки
    }
}

interface AlertPrefsState {
    sound: boolean
    desktop: boolean
    setSound: (value: boolean) => void
    setDesktop: (value: boolean) => void
}

// локальные настройки оповещений устройства: звук и системные уведомления браузера
export const useAlertPrefs = create<AlertPrefsState>((set) => ({
    sound: readFlag(SOUND_KEY, true),
    desktop: readFlag(DESKTOP_KEY, true),
    setSound: (sound) => {
        writeFlag(SOUND_KEY, sound)
        set({ sound })
    },
    setDesktop: (desktop) => {
        writeFlag(DESKTOP_KEY, desktop)
        set({ desktop })
    },
}))

// короткий мягкий сигнал через Web Audio API (без внешних аудиофайлов)
let audioCtx: AudioContext | null = null

export function playNotificationSound() {
    if (!useAlertPrefs.getState().sound) {
        return
    }
    if (typeof window === 'undefined') {
        return
    }
    const Ctx =
        window.AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctx) {
        return
    }
    try {
        audioCtx ??= new Ctx()
        if (audioCtx.state === 'suspended') {
            void audioCtx.resume()
        }
        const now = audioCtx.currentTime
        const osc = audioCtx.createOscillator()
        const gain = audioCtx.createGain()
        osc.type = 'sine'
        osc.frequency.setValueAtTime(660, now)
        osc.frequency.setValueAtTime(880, now + 0.08)
        gain.gain.setValueAtTime(0.0001, now)
        gain.gain.exponentialRampToValueAtTime(0.08, now + 0.02)
        gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.22)
        osc.connect(gain)
        gain.connect(audioCtx.destination)
        osc.start(now)
        osc.stop(now + 0.23)
    } catch {
        // браузер мог заблокировать звук до первого клика
    }
}

export type DesktopPermission = NotificationPermission | 'unsupported'

export function desktopPermission(): DesktopPermission {
    if (typeof Notification === 'undefined') {
        return 'unsupported'
    }
    return Notification.permission
}

export async function requestDesktopPermission(): Promise<DesktopPermission> {
    if (typeof Notification === 'undefined') {
        return 'unsupported'
    }
    try {
        return await Notification.requestPermission()
    } catch {
        return Notification.permission
    }
}

export interface DesktopNoticeInput {
    title: string
    body: string
    tag?: string
    onClick?: () => void
}

// системное уведомление браузера (когда вкладка в фоне и пользователь дал разрешение)
export function showDesktopNotification({ title, body, tag, onClick }: DesktopNoticeInput) {
    if (!useAlertPrefs.getState().desktop) {
        return
    }
    if (typeof Notification === 'undefined' || Notification.permission !== 'granted') {
        return
    }
    try {
        const notice = new Notification(title, {
            body,
            tag,
            icon: '/apple-touch-icon.png',
        })
        notice.onclick = () => {
            window.focus()
            onClick?.()
            notice.close()
        }
    } catch {
        // в некоторых средах конструктор Notification недоступен напрямую
    }
}

// счётчик непрочитанных в заголовке вкладки и точка на favicon
export function updateTabBadge(unreadCount: number) {
    if (typeof document === 'undefined') {
        return
    }
    const count = Math.max(0, unreadCount)
    document.title =
        count > 0 ? `(${count > 99 ? '99+' : count}) ${BASE_TITLE}` : BASE_TITLE

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"][type="image/svg+xml"]')
    if (link) {
        link.href = count > 0 ? BADGED_FAVICON : BASE_FAVICON
    }
}
