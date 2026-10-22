import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import type { ConversationSummary, Member } from '@/entities/conversation'
import { addMessage } from '@/entities/message'
import type { Message } from '@/entities/message'

import { ChatWindow } from './ChatWindow'

const now = new Date().toISOString()

const chat = {
    id: 'c1',
    conversation_type: 'direct',
    peer_id: 'peer',
    my_role: 'member',
    unread_count: 0,
    status: null,
    avatar_shape: null,
    created_at: now,
} as unknown as ConversationSummary

const msg = (id: string, sender: string, content: string | null, extra: Partial<Message> = {}) =>
    ({
        id,
        conversation_id: 'c1',
        sender_id: sender,
        content,
        reply_to_message_id: null,
        is_edited: false,
        is_deleted: false,
        created_at: now,
        updated_at: null,
        ...extra,
    }) as Message

function renderWindow(unread = 0) {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    client.setQueryData(['me'], { user_id: 'me' })
    client.setQueryData(['conversation', 'c1'], { ...chat, unread_count: unread })
    client.setQueryData(['user', 'peer'], {
        user_id: 'peer',
        display_name: 'Дорофей Гришин',
        online: true,
        last_seen_at: null,
    })
    // свежие первыми
    client.setQueryData(['messages', 'c1'], {
        pages: [
            {
                items: [
                    msg('m3', 'me', 'Отвечаю', { reply_to_message_id: 'm1', is_edited: true }),
                    msg('m2', 'peer', null, { is_deleted: true }),
                    msg('m1', 'peer', 'Привет! Как дела?'),
                ],
                limit: 50,
                next_before: null,
            },
        ],
        pageParams: [undefined],
    })
    client.setQueryData<Member[]>(
        ['members', 'c1'],
        [
            { user_id: 'me', last_read_message_id: 'm3' } as Member,
            { user_id: 'peer', last_read_message_id: 'm2' } as Member,
        ],
    )
    return {
        client,
        ...render(
            <QueryClientProvider client={client}>
                <MemoryRouter>
                    <ChatWindow conversationId="c1" />
                </MemoryRouter>
            </QueryClientProvider>,
        ),
    }
}

describe('ChatWindow', () => {
    beforeEach(() => {
        // запросы не уходят в сеть: всё нужное уже в кеше
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('shows the peer, the day and the history', () => {
        renderWindow()
        expect(screen.getByRole('heading', { name: 'Дорофей Гришин' })).toBeInTheDocument()
        expect(screen.getByText('в сети')).toBeInTheDocument()
        expect(screen.getByText('Сегодня')).toBeInTheDocument()
        expect(screen.getAllByText('Привет! Как дела?')).toHaveLength(2) // сообщение и цитата
        expect(screen.getByText('Сообщение удалено')).toBeInTheDocument()
        // вторая копия времени в пузыре - невидимое место под него, скринридеру не видна
        expect(
            screen.getAllByText('изменено').filter((el) => !el.closest('[aria-hidden="true"]')),
        ).toHaveLength(1)
    })

    it('opens actions only on the right click, leaving the left click for selection', () => {
        renderWindow()
        fireEvent.click(screen.getByText('Отвечаю'))
        expect(screen.queryByRole('menu')).toBeNull()
        fireEvent.contextMenu(screen.getByText('Отвечаю'))
        expect(screen.getByRole('menu')).toBeInTheDocument()
    })

    it('offers edit and delete only for my messages', () => {
        renderWindow()
        fireEvent.contextMenu(screen.getByText('Отвечаю'))
        expect(screen.getByRole('menuitem', { name: 'Изменить' })).toBeInTheDocument()
        expect(screen.getByRole('menuitem', { name: 'Удалить' })).toBeInTheDocument()

        // клик по другому сообщению закрывает прежнюю панель (она закрывается по mousedown снаружи)
        const other = screen.getAllByText('Привет! Как дела?').at(-1)!
        fireEvent.mouseDown(other)
        fireEvent.contextMenu(other)
        expect(screen.queryByRole('menuitem', { name: 'Изменить' })).toBeNull()
        expect(screen.getByRole('menuitem', { name: 'Ответить' })).toBeInTheDocument()
    })

    it('puts the reply into the composer', () => {
        renderWindow()
        fireEvent.contextMenu(screen.getAllByText('Привет! Как дела?').at(-1)!)
        fireEvent.click(screen.getByRole('menuitem', { name: 'Ответить' }))
        expect(screen.getByText('Дорофей Гришин:')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Отменить ответ' })).toBeInTheDocument()
    })

    it('puts the unread divider above the first unread message', () => {
        renderWindow(1)
        const divider = screen.getByText('Непрочитанные сообщения')
        // в column-reverse разделитель идёт в разметке сразу после сообщения m1
        expect(divider.previousElementSibling).toHaveAttribute('data-message-id', 'm1')
    })

    it('jumps to the quoted message and highlights it', async () => {
        renderWindow()
        fireEvent.click(
            screen.getByRole('button', { name: 'Перейти к сообщению, на которое ответили' }),
        )
        await vi.waitFor(() => {
            const row = document.querySelector('[data-message-id="m1"]')!
            expect(row.innerHTML).toMatch(/highlighted/)
        })
    })

    it('shows the scroll-down button away from the bottom', () => {
        renderWindow()
        expect(screen.queryByRole('button', { name: 'Вниз' })).toBeNull()
        const log = screen.getByRole('log')
        // в column-reverse вверх от низа scrollTop отрицательный
        Object.defineProperty(log, 'scrollTop', { value: -500, configurable: true })
        log.scrollTo = vi.fn()
        fireEvent.scroll(log)
        fireEvent.click(screen.getByRole('button', { name: 'Вниз' }))
        expect(log.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    })

    it('scrolls to the bottom after my own new message', async () => {
        const { client } = renderWindow()
        const log = screen.getByRole('log')
        log.scrollTo = vi.fn()

        // чужое сообщение прокрутку не трогает
        addMessage(client, { ...msg('m4', 'peer', 'ещё вопрос') })
        await screen.findByText('ещё вопрос')
        expect(log.scrollTo).not.toHaveBeenCalled()

        addMessage(client, { ...msg('temp-1', 'me', 'моё'), pending: true })
        await screen.findByText('моё')
        expect(log.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    })

    it('marks as read only up to the freshest message that was on screen', async () => {
        const fetchMock = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(
            () => new Promise(() => {}),
        )
        vi.stubGlobal('fetch', fetchMock)
        // на экране только старое сообщение m1, свежие m2 и m3 ниже видимой области
        class PartialObserver {
            constructor(
                private callback: (entries: Partial<IntersectionObserverEntry>[]) => void,
            ) {}
            observe(target: HTMLElement) {
                const visible = target.dataset.messageId === 'm1'
                this.callback([
                    {
                        target,
                        isIntersecting: visible,
                        intersectionRatio: visible ? 1 : 0,
                        intersectionRect: { height: visible ? 40 : 0 } as DOMRectReadOnly,
                    },
                ])
            }
            disconnect() {}
        }
        vi.stubGlobal('IntersectionObserver', PartialObserver)

        renderWindow(2)
        await vi.waitFor(() =>
            expect(fetchMock).toHaveBeenCalledWith(
                '/api/conversations/c1/read',
                expect.objectContaining({ method: 'POST', body: '{"message_id":"m1"}' }),
            ),
        )
        expect(
            fetchMock.mock.calls.some(
                ([url, init]) => url.endsWith('/read') && String(init?.body).includes('m3'),
            ),
        ).toBe(false)
    })
})
