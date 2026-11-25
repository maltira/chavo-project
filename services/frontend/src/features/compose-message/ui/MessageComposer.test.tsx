import { act, fireEvent, screen, within } from '@testing-library/react'

import { useDrafts } from '@/entities/message'
import type { Message } from '@/entities/message'
import { selectionOffsets, setSelectionOffsets } from '@/shared/lib'
import { renderWithProviders, typeMessage } from '@/shared/lib/testing'

import { MessageComposer } from './MessageComposer'

const fetchMock = vi.fn()

// выделение в поле и событие, по которому появляется панель форматирования
function select(field: HTMLElement, start: number, end: number) {
    setSelectionOffsets(field, start, end)
    act(() => {
        document.dispatchEvent(new Event('selectionchange'))
    })
}

const body = (call = 0) => JSON.parse(fetchMock.mock.calls[call]![1].body as string)

describe('MessageComposer', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response(JSON.stringify({ id: 'm1' }), { status: 201 }))
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        useDrafts.getState().clearAll()
    })

    it('sends on Enter, keeps Shift+Enter as a line break and clears the field', async () => {
        const onCancel = vi.fn()
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={onCancel} />)
        const field = screen.getByLabelText('Сообщение')
        expect(screen.getByRole('button', { name: 'Отправить' })).toBeDisabled()

        typeMessage(field, '  привет  ')
        fireEvent.keyDown(field, { key: 'Enter', shiftKey: true })
        expect(fetchMock).not.toHaveBeenCalled()

        fireEvent.keyDown(field, { key: 'Enter' })
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
        const [url] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/messages')
        // текст без пробелов и переносов по краям, без форматирования - без поля entities
        expect(body()).toEqual({
            conversation_id: 'c1',
            content: 'привет',
            reply_to_message_id: null,
        })
        expect(field).toHaveAttribute('data-empty', 'true')
        expect(field).toHaveTextContent('')
    })

    it('sends the formatting with the text, shifted by the trimmed edge', async () => {
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />)
        const field = screen.getByLabelText('Сообщение')
        typeMessage(field, '  привет мир')
        // «привет» - жирный, «мир» - подчёркнутый
        select(field, 2, 8)
        fireEvent.keyDown(field, { code: 'KeyB', ctrlKey: true, key: 'b' })
        select(field, 9, 12)
        fireEvent.click(screen.getByRole('button', { name: 'Подчёркнутый' }))
        expect(field.querySelector('b')?.textContent).toBe('привет')

        fireEvent.keyDown(field, { key: 'Enter' })
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
        expect(body()).toMatchObject({
            content: 'привет мир',
            entities: [
                { type: 'bold', offset: 0, length: 6 },
                { type: 'underline', offset: 7, length: 3 },
            ],
        })
        // после отправки поле чистое, и новое сообщение не наследует форматирование
        expect(field.innerHTML).toBe('')
    })

    it('does not grab the focus when a chat is just opened', () => {
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />)
        expect(screen.getByLabelText('Сообщение')).not.toHaveFocus()
    })

    it('sends a reply to the chosen message', async () => {
        renderWithProviders(
            <MessageComposer
                conversationId="c1"
                myId="me"
                replyTo={{ id: 'm9', author: 'Дорофей', text: 'вопрос' }}
                onCancel={vi.fn()}
            />,
        )
        expect(screen.getByText('Дорофей:')).toBeInTheDocument()
        expect(screen.getByLabelText('Сообщение')).toHaveFocus()
        typeMessage(screen.getByLabelText('Сообщение'), 'ответ')
        fireEvent.click(screen.getByRole('button', { name: 'Отправить' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        expect(body().reply_to_message_id).toBe('m9')
    })

    it('edits a message and saves only a changed text', async () => {
        const editing = { id: 'm5', content: 'старый текст' } as Message
        renderWithProviders(
            <MessageComposer conversationId="c1" myId="me" editing={editing} onCancel={vi.fn()} />,
        )
        const field = screen.getByLabelText('Сообщение')
        expect(field).toHaveTextContent('старый текст')
        // курсор стоит в конце текста, а не в начале
        const end = 'старый текст'.length
        expect(selectionOffsets(field)).toEqual({ start: end, end })
        expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()

        typeMessage(field, 'новый текст')
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/messages/m5')
        expect(init.method).toBe('PATCH')
        // правка заменяет и разметку: без форматирования уходит пустой список
        expect(body()).toEqual({ content: 'новый текст', entities: [] })
    })

    it('opens a formatted message with its formatting and saves a change of the formatting alone', async () => {
        const editing = {
            id: 'm6',
            content: 'привет мир',
            entities: [{ type: 'bold', offset: 0, length: 6 }],
        } as Message
        renderWithProviders(
            <MessageComposer conversationId="c1" myId="me" editing={editing} onCancel={vi.fn()} />,
        )
        const field = screen.getByLabelText('Сообщение')
        expect(field.querySelector('b')?.textContent).toBe('привет')
        // ничего не менялось - сохранять нечего
        expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()

        // текст тот же, но добавили курсив - это уже изменение
        select(field, 7, 10)
        fireEvent.keyDown(field, { code: 'KeyI', ctrlKey: true, key: 'i' })
        expect(screen.getByRole('button', { name: 'Сохранить' })).toBeEnabled()
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        expect(body()).toEqual({
            content: 'привет мир',
            entities: [
                { type: 'bold', offset: 0, length: 6 },
                { type: 'italic', offset: 7, length: 3 },
            ],
        })
    })

    it('does not send a message that has only spaces, even if they are formatted', () => {
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />)
        const field = screen.getByLabelText('Сообщение')
        typeMessage(field, '   ')
        select(field, 0, 3)
        fireEvent.keyDown(field, { code: 'KeyB', ctrlKey: true, key: 'b' })
        expect(screen.getByRole('button', { name: 'Отправить' })).toBeDisabled()
        fireEvent.keyDown(field, { key: 'Enter' })
        expect(fetchMock).not.toHaveBeenCalled()
    })

    describe('emoji panel', () => {
        const open = async () => {
            fireEvent.click(screen.getByRole('button', { name: 'Эмодзи' }))
            return screen.findByRole('region', { name: 'Смайлы и люди' })
        }
        const firstEmoji = (region: HTMLElement) =>
            within(region).getAllByRole('button')[0]!.getAttribute('data-emoji')!

        it('opens by the button, inserts at the caret and stays open for the next one', async () => {
            // настройки грузятся при открытии панели
            fetchMock.mockResolvedValue(
                new Response(JSON.stringify({ favorite_emojis: [] }), { status: 200 }),
            )
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            const field = screen.getByLabelText('Сообщение')
            typeMessage(field, 'приветмир')
            select(field, 6, 6)

            const people = await open()
            const emoji = firstEmoji(people)
            // мышью (detail 1): фокус переходит в поле, можно печатать дальше
            fireEvent.click(within(people).getAllByRole('button')[0]!, { detail: 1 })
            expect(field).toHaveTextContent(`привет${emoji}мир`)
            expect(field).toHaveFocus()
            // панель осталась открытой, курсор - сразу после эмодзи
            expect(screen.getByRole('dialog', { name: 'Эмодзи' })).toBeInTheDocument()
            await vi.waitFor(() => expect(selectionOffsets(field)?.start).toBe(6 + emoji.length))
            // сообщение с эмодзи отправляется как обычный текст
            expect(screen.getByRole('button', { name: 'Отправить' })).toBeEnabled()
        })

        it('keeps the focus in the panel after a pick with the keyboard', async () => {
            fetchMock.mockResolvedValue(new Response(JSON.stringify({ favorite_emojis: [] })))
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            const field = screen.getByLabelText('Сообщение')
            typeMessage(field, 'ab')
            select(field, 2, 2)
            const people = await open()
            const first = within(people).getAllByRole('button')[0]!
            first.focus()
            // Enter на кнопке даёт click с detail 0: вставка есть, а фокус остаётся на эмодзи
            fireEvent.click(first, { detail: 0 })
            expect(field.textContent).toContain(firstEmoji(people))
            expect(first).toHaveFocus()
        })

        it('replaces the selected text and closes with Esc or a click outside', async () => {
            fetchMock.mockResolvedValue(new Response(JSON.stringify({ favorite_emojis: [] })))
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            const field = screen.getByLabelText('Сообщение')
            typeMessage(field, 'abcdef')
            select(field, 1, 4)
            const people = await open()
            const emoji = firstEmoji(people)
            fireEvent.click(within(people).getAllByRole('button')[0]!)
            expect(field).toHaveTextContent(`a${emoji}ef`)

            fireEvent.keyDown(document, { key: 'Escape' })
            expect(screen.queryByRole('dialog', { name: 'Эмодзи' })).toBeNull()

            await open()
            fireEvent.mouseDown(field)
            expect(screen.queryByRole('dialog', { name: 'Эмодзи' })).toBeNull()
        })

        it('does not read the settings until the panel is opened', () => {
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            expect(fetchMock).not.toHaveBeenCalled()
        })

        it('saves a favorite on the server and shows it at once', async () => {
            fetchMock.mockImplementation((_url: string, init?: RequestInit) =>
                Promise.resolve(
                    new Response(
                        JSON.stringify({
                            favorite_emojis: init?.method === 'PATCH' ? ['🔥'] : [],
                        }),
                    ),
                ),
            )
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            const people = await open()
            fireEvent.contextMenu(within(people).getAllByRole('button')[0]!)
            await vi.waitFor(() =>
                expect(fetchMock).toHaveBeenCalledWith(
                    '/api/users/me/settings',
                    expect.objectContaining({ method: 'PATCH' }),
                ),
            )
            const patch = fetchMock.mock.calls.find(([, init]) => init?.method === 'PATCH')!
            expect(JSON.parse(patch[1].body as string)).toEqual({
                favorite_emojis: [firstEmoji(people)],
            })
        })
    })

    it('restores a saved draft with formatting on mount and clears it on send', async () => {
        useDrafts.getState().setDraft('c1', {
            text: 'незаконченный текст',
            entities: [{ type: 'bold', offset: 0, length: 13 }],
        })
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />)
        const field = screen.getByLabelText('Сообщение')
        expect(field).toHaveTextContent('незаконченный текст')
        expect(field.querySelector('b')?.textContent).toBe('незаконченный')

        fireEvent.keyDown(field, { key: 'Enter' })
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
        expect(useDrafts.getState().items.c1).toBeUndefined()
    })
})
