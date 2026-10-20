import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import { setAccessToken } from '@/shared/api'

import { SettingsModal } from './SettingsModal'

const fetchMock = vi.fn()

// access token с sid текущего сеанса
const token = `x.${btoa(JSON.stringify({ sid: 's-current' }))}.y`

// первая страница чёрного списка; полная (50) - значит, на сервере есть ещё
let blockedPage: object[] = []

function renderSettings() {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], {
        user_id: 'me',
        display_name: 'Андрей',
        username: 'enemybye',
        status: null,
    })
    client.setQueryData(
        ['sessions'],
        [
            {
                id: 's-current',
                device_name: 'Firefox, Linux',
                created_at: new Date().toISOString(),
            },
            {
                id: 's-phone',
                device_name: 'iPhone 11',
                ip_address: '10.0.0.7',
                created_at: new Date().toISOString(),
            },
            { id: 's-tablet', device_name: 'iPad', created_at: new Date().toISOString() },
        ],
    )
    client.setQueryData(['blocked'], { pages: [blockedPage], pageParams: [0] })
    client.setQueryData(['settings'], { allow_group_invites: true, show_online_status: true })
    render(
        <QueryClientProvider client={client}>
            <MemoryRouter>
                <SettingsModal open onClose={vi.fn()} />
            </MemoryRouter>
        </QueryClientProvider>,
    )
}

describe('SettingsModal', () => {
    beforeEach(() => {
        blockedPage = [{ blocked_user_id: 'u1', display_name: 'Давыд', username: 'davyd' }]
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        setAccessToken(token)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        setAccessToken(null)
    })

    it('shows me, counters and the empty status, and goes back from a section', () => {
        renderSettings()
        expect(screen.getByRole('heading', { name: 'Андрей' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Установить статус' })).toBeInTheDocument()
        expect(screen.getByRole('menuitem', { name: /Активные сеансы\s*3/ })).toBeInTheDocument()
        expect(screen.getByRole('menuitem', { name: /Чёрный список\s*1/ })).toBeInTheDocument()

        fireEvent.click(screen.getByRole('menuitem', { name: 'Параметры интерфейса' }))
        expect(screen.getByRole('dialog', { name: 'Настройки интерфейса' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(screen.getByRole('heading', { name: 'Андрей' })).toBeInTheDocument()
    })

    it('opens the status editor as a screen of the same window and goes back', () => {
        renderSettings()
        fireEvent.click(screen.getByRole('button', { name: 'Установить статус' }))
        // одно окно: редактор заменил главный экран, а не открылся поверх
        expect(screen.getAllByRole('dialog')).toHaveLength(1)
        expect(screen.getByRole('dialog', { name: 'Статус' })).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(screen.getByRole('heading', { name: 'Андрей' })).toBeInTheDocument()
    })

    it('saves privacy settings', async () => {
        fetchMock.mockResolvedValue(
            new Response(JSON.stringify({ allow_group_invites: false, show_online_status: true })),
        )
        renderSettings()
        fireEvent.click(screen.getByRole('menuitem', { name: 'Конфиденциальность' }))
        const save = screen.getByRole('button', { name: 'Сохранить изменения' })
        expect(save).toBeDisabled()
        fireEvent.click(screen.getByRole('button', { name: /Кто может добавлять меня/ }))
        fireEvent.click(save)
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/users/me/settings')
        expect(JSON.parse(init.body as string)).toEqual({
            allow_group_invites: false,
            show_online_status: true,
        })
    })

    it('ends every session except the current one', async () => {
        renderSettings()
        fireEvent.click(screen.getByRole('menuitem', { name: /Активные сеансы/ }))
        expect(screen.getByText('Текущий сеанс')).toBeInTheDocument()
        expect(screen.getByText('10.0.0.7')).toBeInTheDocument()

        fireEvent.click(screen.getByRole('button', { name: 'Завершить все, кроме текущего' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Завершить' }))

        // после завершения список сеансов перечитывается: считаем только удаления
        const deleted = () =>
            fetchMock.mock.calls.filter(([, init]) => init?.method === 'DELETE').map(([url]) => url)
        await vi.waitFor(() => expect(deleted()).toHaveLength(2))
        const urls = deleted()
        expect(urls).toEqual(
            expect.arrayContaining(['/api/auth/sessions/s-phone', '/api/auth/sessions/s-tablet']),
        )
        expect(urls).not.toContain('/api/auth/sessions/s-current')
    })

    it('unblocks a person from the black list', async () => {
        renderSettings()
        fireEvent.click(screen.getByRole('menuitem', { name: /Чёрный список/ }))
        fireEvent.click(screen.getByRole('button', { name: 'Разблокировать Давыд' }))
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/users/u1/block',
                expect.objectContaining({ method: 'DELETE' }),
            ),
        )
    })

    it('unblocks everybody, loading the pages that were not shown yet', async () => {
        blockedPage = Array.from({ length: 50 }, (_, i) => ({
            blocked_user_id: `p1-${i}`,
            display_name: `Первый ${i}`,
            username: `first${i}`,
        }))
        fetchMock.mockImplementation((url: string) => {
            if (url.includes('/users/me/blocked') && url.includes('offset=50')) {
                const items = [
                    { blocked_user_id: 'p2-0', display_name: 'Второй', username: 'second' },
                ]
                return Promise.resolve(new Response(JSON.stringify({ items })))
            }
            return Promise.resolve(new Response('{"message":"ok"}'))
        })
        renderSettings()
        // в счётчике видно, что загружено не всё
        expect(screen.getByRole('menuitem', { name: /Чёрный список\s*50\+/ })).toBeInTheDocument()

        fireEvent.click(screen.getByRole('menuitem', { name: /Чёрный список/ }))
        fireEvent.click(screen.getByRole('button', { name: 'Разблокировать всех' }))
        const confirm = screen.getByRole('dialog', { name: 'Вы уверены?' })
        fireEvent.click(within(confirm).getByRole('button', { name: 'Разблокировать' }))

        const unblocked = () =>
            fetchMock.mock.calls
                .filter(([, init]) => init?.method === 'DELETE')
                .map(([url]) => String(url))
        await vi.waitFor(() => expect(unblocked()).toHaveLength(51))
        expect(unblocked()).toContain('/api/users/p2-0/block')
    })
})
