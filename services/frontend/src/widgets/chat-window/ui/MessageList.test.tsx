import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'

import type { ConversationSummary } from '@/entities/conversation'
import type { Message } from '@/entities/message'
import { useProfileCard } from '@/entities/user'

import { MessageList } from './MessageList'

const now = Date.now()
// свежие первыми; at - сколько минут назад
const msg = (id: string, sender: string, at: number) =>
    ({
        id,
        conversation_id: 'g1',
        sender_id: sender,
        content: id,
        reply_to_message_id: null,
        is_edited: false,
        is_deleted: false,
        created_at: new Date(now - at * 60_000).toISOString(),
        updated_at: null,
    }) as Message

function renderList(
    conversation_type: 'group' | 'direct',
    options: { saved?: boolean; messages?: Message[] } = {},
) {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } })
    client.setQueryData(['user', 'a'], { user_id: 'a', display_name: 'Давыд' })
    client.setQueryData(['user', 'b'], { user_id: 'b', display_name: 'Герман' })
    render(
        <QueryClientProvider client={client}>
            <MessageList
                chat={
                    {
                        id: 'g1',
                        conversation_type,
                        my_role: 'member',
                        is_saved: !!options.saved,
                    } as ConversationSummary
                }
                // снизу вверх: a (2 подряд), моё, b, a
                messages={
                    options.messages ?? [
                        msg('a3', 'a', 1),
                        msg('a2', 'a', 2),
                        msg('mine', 'me', 3),
                        msg('b1', 'b', 4),
                        msg('a1', 'a', 5),
                    ]
                }
                myId="me"
                readBoundary={undefined}
                hasOlder={false}
                loadingOlder={false}
                loadOlder={vi.fn()}
                firstUnreadId={null}
                findMessage={vi.fn()}
                onSeen={vi.fn()}
                onReply={vi.fn()}
                onEdit={vi.fn()}
                onDelete={vi.fn()}
                pins={[]}
                onPin={vi.fn()}
                jumpRequest={null}
                onForward={vi.fn()}
                selected={null}
                onToggleSelect={vi.fn()}
            />
        </QueryClientProvider>,
    )
}

describe('MessageList avatars', () => {
    beforeEach(() => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('shows the author avatar only under the last message of a series in a group', () => {
        renderList('group')
        // серия a3+a2 - один аватар, b1 и a1 - по одному; у своего сообщения аватара нет
        expect(screen.getAllByRole('button', { name: 'Профиль: Давыд' })).toHaveLength(2)
        expect(screen.getAllByRole('button', { name: 'Профиль: Герман' })).toHaveLength(1)
        const series = document.querySelector('[data-message-id="a3"]')!
        expect(series.querySelector('button[aria-label^="Профиль"]')).not.toBeNull()
        const above = document.querySelector('[data-message-id="a2"]')!
        expect(above.querySelector('button[aria-label^="Профиль"]')).toBeNull()
    })

    it('pulls a message closer to the previous one of the same author', () => {
        renderList('group')
        // a3 идёт сразу за a2 того же автора, a2 - после моего сообщения
        expect(document.querySelector('[data-message-id="a3"]')!.className).toMatch(/continued/)
        expect(document.querySelector('[data-message-id="a2"]')!.className).not.toMatch(/continued/)
    })

    it('puts forwarded messages with an author on the left in «Избранное»', () => {
        renderList('direct', {
            saved: true,
            messages: [
                { ...msg('plain', 'me', 1), is_forwarded: false },
                { ...msg('fwd2', 'me', 2), is_forwarded: true, forwarded_from_user_id: 'a' },
                { ...msg('fwd1', 'me', 3), is_forwarded: true, forwarded_from_user_id: 'a' },
                msg('note', 'me', 4),
            ],
        })
        const row = (id: string) => document.querySelector(`[data-message-id="${id}"]`)!
        expect(row('note').className).toMatch(/mine/)
        expect(row('fwd1').className).not.toMatch(/mine/)
        expect(row('fwd2').className).not.toMatch(/mine/)
        // пересланные одного автора - одна серия, моё после них - новая
        expect(row('fwd2').className).toMatch(/continued/)
        expect(row('plain').className).toMatch(/mine/)
        expect(row('plain').className).not.toMatch(/continued/)
        // аватар первого автора - у нижнего сообщения его серии, у своих аватара нет
        expect(
            within(row('fwd2') as HTMLElement).getByRole('button', { name: 'Профиль: Давыд' }),
        ).toBeInTheDocument()
        expect(
            within(row('fwd1') as HTMLElement).queryByRole('button', { name: /Профиль/ }),
        ).toBeNull()
        expect(
            within(row('note') as HTMLElement).queryByRole('button', { name: /Профиль/ }),
        ).toBeNull()
        // своё пересланное слева, но удалить его можно, а править - нет
        fireEvent.contextMenu(screen.getByText('fwd1'))
        expect(screen.getByRole('menuitem', { name: 'Удалить' })).toBeInTheDocument()
        expect(screen.queryByRole('menuitem', { name: 'Изменить' })).toBeNull()
    })

    it('has no avatars in a direct chat', () => {
        renderList('direct')
        expect(screen.queryByRole('button', { name: /^Профиль/ })).toBeNull()
    })
})

describe('MessageList system messages', () => {
    beforeEach(() => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('shows membership events as centered lines with current names', () => {
        const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } })
        client.setQueryData(['user', 'a'], { user_id: 'a', display_name: 'Давыд' })
        client.setQueryData(['user', 'b'], { user_id: 'b', display_name: 'Герман' })
        const system = (id: string, event: string, sender: string, target: string, at: number) =>
            ({
                ...msg(id, sender, at),
                content: null,
                kind: 'system',
                system_event: event,
                target_id: target,
            }) as Message
        render(
            <QueryClientProvider client={client}>
                <MessageList
                    chat={
                        {
                            id: 'g1',
                            conversation_type: 'group',
                            my_role: 'member',
                        } as ConversationSummary
                    }
                    messages={[
                        system('s4', 'removed', 'a', 'b', 1),
                        system('s3', 'left', 'b', 'b', 2),
                        system('s2', 'joined', 'b', 'b', 3),
                        system('s1', 'added', 'a', 'b', 4),
                        msg('a1', 'a', 5),
                    ]}
                    myId="me"
                    readBoundary={undefined}
                    hasOlder={false}
                    loadingOlder={false}
                    loadOlder={vi.fn()}
                    firstUnreadId={null}
                    findMessage={vi.fn()}
                    onSeen={vi.fn()}
                    onReply={vi.fn()}
                    onEdit={vi.fn()}
                    onDelete={vi.fn()}
                    pins={[]}
                    onPin={vi.fn()}
                    jumpRequest={null}
                    onForward={vi.fn()}
                    selected={null}
                    onToggleSelect={vi.fn()}
                />
            </QueryClientProvider>,
        )
        const lines = [...document.querySelectorAll('[data-message-id^="s"]')].map(
            (line) => line.textContent,
        )
        expect(lines).toEqual([
            'Давыд исключил Герман',
            'Герман покинул чат',
            'Герман присоединился к чату',
            'Давыд добавил Герман',
        ])

        // имена - ссылки на профили
        fireEvent.click(screen.getAllByRole('button', { name: 'Профиль: Герман' })[0]!)
        expect(useProfileCard.getState().userId).toBe('b')
        // обычное сообщение по-прежнему пузырём
        expect(screen.getByText('a1')).toBeInTheDocument()
    })
})

describe('MessageList appearing messages', () => {
    beforeEach(() => {
        vi.stubGlobal(
            'fetch',
            vi.fn(() => new Promise(() => {})),
        )
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('animates only messages that arrive while the chat is open', () => {
        const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } })
        const list = (messages: Message[]) => (
            <QueryClientProvider client={client}>
                <MessageList
                    chat={
                        {
                            id: 'g1',
                            conversation_type: 'direct',
                            my_role: 'member',
                        } as ConversationSummary
                    }
                    messages={messages}
                    myId="me"
                    readBoundary={undefined}
                    hasOlder={false}
                    loadingOlder={false}
                    loadOlder={vi.fn()}
                    firstUnreadId={null}
                    findMessage={vi.fn()}
                    onSeen={vi.fn()}
                    onReply={vi.fn()}
                    onEdit={vi.fn()}
                    onDelete={vi.fn()}
                    pins={[]}
                    onPin={vi.fn()}
                    jumpRequest={null}
                    onForward={vi.fn()}
                    selected={null}
                    onToggleSelect={vi.fn()}
                />
            </QueryClientProvider>
        )
        const row = (id: string) => document.querySelector(`[data-message-id="${id}"]`)!
        const opened = [msg('b2', 'b', 5), msg('b1', 'b', 6)]
        const { rerender } = render(list(opened))
        expect(row('b2').className).not.toMatch(/appear/)

        // своё неподтверждённое и подгруженная старая страница
        const temp = { ...msg('temp-1', 'me', 0), pending: true }
        rerender(list([temp, ...opened, msg('old', 'b', 60)]))
        expect(row('temp-1').className).toMatch(/appear/)
        // новая строка раскрывается по высоте в обёртке, история лежит без неё
        expect(row('temp-1').parentElement!.parentElement!.className).toMatch(/grow/)
        expect(row('b2').parentElement!.className).not.toMatch(/grow|inner/)
        expect(row('old').className).not.toMatch(/appear/)

        // ответ сервера заменяет временное, но строка остаётся той же - второй анимации нет
        const before = row('temp-1')
        rerender(
            list([{ ...msg('m1', 'me', 0), local_key: 'temp-1' }, ...opened, msg('old', 'b', 60)]),
        )
        expect(row('m1')).toBe(before)
    })
})
