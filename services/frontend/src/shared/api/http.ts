import { getAccessToken, setAccessToken } from './token'
import type { ErrorResponse } from './types'

const BASE_URL = '/api'

// ошибка сервера в формате {error, reason}; reason - машиночитаемый код
export class ApiError extends Error {
    readonly status: number
    readonly reason?: string

    constructor(status: number, message: string, reason?: string) {
        super(message)
        this.name = 'ApiError'
        this.status = status
        this.reason = reason
    }
}

interface RequestOptions {
    method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
    body?: unknown
}

let onSessionLost: (() => void) | null = null

// хранилище сессии регистрирует себя здесь: shared не знает про слои выше
export function setSessionLostHandler(handler: (() => void) | null) {
    onSessionLost = handler
}

async function toApiError(response: Response) {
    let message = 'Не удалось выполнить запрос'
    let reason: string | undefined
    try {
        const data = (await response.json()) as ErrorResponse
        message = data.error || message
        reason = data.reason
    } catch {
        // тело не JSON (например, ответ прокси)
    }
    return new ApiError(response.status, message, reason)
}

// исход обновления токена; от него зависит, потеряна ли сессия
//  ok          - новый токен получен
//  rejected    - сервер ответил, что сессии нет (cookie нет или недействительна)
//  unavailable - ответа нет: сеть пропала или сервис недоступен, сессия может быть жива
export type RefreshResult =
    { status: 'ok'; token: string } | { status: 'rejected' } | { status: 'unavailable' }

let refreshing: Promise<RefreshResult> | null = null

// один общий refresh на все параллельные запросы
export function refreshAccessToken(): Promise<RefreshResult> {
    refreshing ??= doRefresh().finally(() => {
        refreshing = null
    })
    return refreshing
}

async function doRefresh(): Promise<RefreshResult> {
    let response: Response
    try {
        response = await fetch(`${BASE_URL}/auth/refresh`, {
            method: 'POST',
            credentials: 'same-origin',
        })
    } catch {
        return { status: 'unavailable' }
    }

    // 5xx и 429 - проблема сервиса
    if (response.status >= 500 || response.status === 429) {
        return { status: 'unavailable' }
    }
    if (!response.ok) {
        return { status: 'rejected' }
    }

    try {
        const data = (await response.json()) as { access_token?: string }
        if (data.access_token) {
            setAccessToken(data.access_token)
            return { status: 'ok', token: data.access_token }
        }
    } catch {
        // тело не JSON - ниже считаем отказом
    }
    return { status: 'rejected' }
}

function networkError() {
    return new ApiError(0, 'Не удалось выполнить запрос, проверьте соединение', 'NETWORK_ERROR')
}

function send(path: string, { method = 'GET', body }: RequestOptions) {
    const headers: Record<string, string> = {}
    const token = getAccessToken()
    // FormData (загрузка файла) уходит как есть: Content-Type с boundary ставит браузер.
    // Её можно отправить повторно, поэтому повтор после обновления токена тоже работает
    const isForm = body instanceof FormData
    if (body !== undefined && !isForm) {
        headers['Content-Type'] = 'application/json'
    }
    if (token) {
        headers.Authorization = `Bearer ${token}`
    }
    return fetch(`${BASE_URL}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
        credentials: 'same-origin',
    })
}

async function parse<T>(response: Response): Promise<T> {
    if (response.status === 204) {
        return undefined as T
    }
    const text = await response.text()
    return (text ? JSON.parse(text) : undefined) as T
}

// fetch бросает исключение только при сбое сети: превращаем его в ApiError с понятным кодом
async function sendSafe(path: string, options: RequestOptions) {
    try {
        return await send(path, options)
    } catch {
        throw networkError()
    }
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    let response = await sendSafe(path, options)

    // 401 вне /auth/*: пробуем обновить токен один раз и повторяем запрос
    if (response.status === 401 && !path.startsWith('/auth/')) {
        const refresh = await refreshAccessToken()
        if (refresh.status === 'rejected') {
            // сервер подтвердил, что сессии нет - выходим
            setAccessToken(null)
            onSessionLost?.()
            throw await toApiError(response)
        }
        if (refresh.status === 'unavailable') {
            // сессия может быть жива: не выходим, следующий запрос повторит попытку
            throw networkError()
        }
        response = await sendSafe(path, options)
    }

    if (!response.ok) {
        throw await toApiError(response)
    }
    return parse<T>(response)
}
