import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'

import { useSession } from '@/entities/session'

import { AppShell } from './AppShell'

// запрос выхода подменяется глобальным fetch: тест не лезет внутрь фичи в обход публичного api
const fetchMock = vi.fn()

function renderShell() {
    // данные профиля заданы заранее и считаются свежими: лишнего запроса /users/me не будет
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], {
        user_id: 'u1',
        username: 'enemybye',
        display_name: 'Your Name',
        avatar_shape: { shape: 'flower', color: '#D97757' },
    })
    return render(
        <QueryClientProvider client={client}>
            <AppShell list={(compact) => <p>список: {compact ? 'свёрнут' : 'полный'}</p>}>
                <p>панель</p>
            </AppShell>
        </QueryClientProvider>,
    )
}

describe('AppShell', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response('{"message":"ok"}'))
        vi.stubGlobal('fetch', fetchMock)
        useSession.getState().signIn('tok')
        document.documentElement.dataset.theme = 'dark'
    })

    it('shows the sidebar controls, the list and the panel', () => {
        renderShell()
        expect(screen.getByLabelText('Поиск')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Новый чат' })).toBeInTheDocument()
        expect(screen.getByText('список: полный')).toBeInTheDocument()
        expect(screen.getByText('панель')).toBeInTheDocument()
    })

    it('shows the profile and the actions in the menu', () => {
        renderShell()
        expect(screen.queryByRole('menu')).toBeNull()

        fireEvent.click(screen.getByRole('button', { name: 'Меню' }))
        expect(screen.getByText('Your Name')).toBeInTheDocument()
        expect(screen.getByText('@enemybye')).toBeInTheDocument()
        expect(screen.getByRole('menuitem', { name: 'Выйти из аккаунта' })).toBeInTheDocument()
    })

    it('switches the theme from the menu', () => {
        renderShell()
        fireEvent.click(screen.getByRole('button', { name: 'Меню' }))
        fireEvent.click(screen.getByRole('switch', { name: 'Тёмная тема' }))
        expect(document.documentElement.dataset.theme).toBe('light')
    })

    it('closes the menu on Escape', () => {
        renderShell()
        fireEvent.click(screen.getByRole('button', { name: 'Меню' }))
        fireEvent.keyDown(document, { key: 'Escape' })
        // меню играет анимацию закрытия и убирается по её окончанию
        expect(screen.getByRole('menu')).toHaveAttribute('data-state', 'closed')
        // в jsdom нет AnimationEvent, и React слушает событие под префиксным именем
        fireEvent(screen.getByRole('menu'), new Event('webkitAnimationEnd', { bubbles: true }))
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('logs out from the menu', async () => {
        renderShell()
        fireEvent.click(screen.getByRole('button', { name: 'Меню' }))
        fireEvent.click(screen.getByRole('menuitem', { name: 'Выйти из аккаунта' }))

        await vi.waitFor(() => expect(useSession.getState().status).toBe('guest'))
        expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/logout')
    })
})

describe('AppShell sidebar width', () => {
    beforeEach(() => {
        localStorage.clear()
        useSession.getState().signIn('tok')
    })

    function sidebar() {
        return screen.getByRole('complementary')
    }
    const resizer = () => screen.getByRole('separator', { name: 'Ширина списка чатов' })

    it('starts with the default width', () => {
        renderShell()
        expect(sidebar()).toHaveStyle({ width: '362px' })
    })

    it('restores the saved width', () => {
        localStorage.setItem('chavo:sidebar-width', '400')
        renderShell()
        expect(sidebar()).toHaveStyle({ width: '400px' })
    })

    it('ignores a broken saved value', () => {
        localStorage.setItem('chavo:sidebar-width', 'abc')
        renderShell()
        expect(sidebar()).toHaveStyle({ width: '362px' })
    })

    it('resizes by dragging and saves the result when released', () => {
        renderShell()
        fireEvent.pointerDown(resizer(), { clientX: 362, button: 0, pointerId: 1 })
        fireEvent.pointerMove(resizer(), { clientX: 420, pointerId: 1 })
        expect(sidebar()).toHaveStyle({ width: '420px' })
        expect(localStorage.getItem('chavo:sidebar-width')).toBeNull()

        fireEvent.pointerUp(resizer(), { clientX: 420, pointerId: 1 })
        expect(localStorage.getItem('chavo:sidebar-width')).toBe('420')
    })

    it('never goes wider than the maximum', () => {
        renderShell()
        fireEvent.pointerDown(resizer(), { clientX: 362, button: 0, pointerId: 1 })
        fireEvent.pointerMove(resizer(), { clientX: 900, pointerId: 1 })
        fireEvent.pointerUp(resizer(), { clientX: 900, pointerId: 1 })
        expect(sidebar()).toHaveStyle({ width: '450px' })
    })

    it('turns into the compact version below the threshold and stays there', () => {
        renderShell()
        fireEvent.pointerDown(resizer(), { clientX: 362, button: 0, pointerId: 1 })
        fireEvent.pointerMove(resizer(), { clientX: 120, pointerId: 1 })
        fireEvent.pointerUp(resizer(), { clientX: 120, pointerId: 1 })

        expect(sidebar()).toHaveStyle({ width: '76px' })
        expect(screen.queryByLabelText('Поиск')).toBeNull()
        expect(screen.queryByRole('button', { name: 'Новый чат' })).toBeNull()
        expect(screen.getByRole('button', { name: 'Меню' })).toBeInTheDocument()

        // дальше влево не уменьшается
        fireEvent.pointerDown(resizer(), { clientX: 76, button: 0, pointerId: 1 })
        fireEvent.pointerMove(resizer(), { clientX: 10, pointerId: 1 })
        fireEvent.pointerUp(resizer(), { clientX: 10, pointerId: 1 })
        expect(sidebar()).toHaveStyle({ width: '76px' })
    })

    it('is not a keyboard control: not focusable and ignores arrow keys', () => {
        renderShell()
        expect(resizer()).not.toHaveAttribute('tabindex')

        fireEvent.keyDown(resizer(), { key: 'ArrowRight' })
        fireEvent.keyDown(resizer(), { key: 'Home' })
        expect(sidebar()).toHaveStyle({ width: '362px' })
    })

    it('goes back to the default width on double click', () => {
        localStorage.setItem('chavo:sidebar-width', '300')
        renderShell()
        fireEvent.doubleClick(resizer())
        expect(sidebar()).toHaveStyle({ width: '362px' })
        expect(localStorage.getItem('chavo:sidebar-width')).toBe('362')
    })
})
