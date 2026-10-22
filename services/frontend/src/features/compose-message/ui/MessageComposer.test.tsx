import { fireEvent, screen, within } from '@testing-library/react'

import type { Message } from '@/entities/message'
import { renderWithProviders } from '@/shared/lib/testing'

import { MessageComposer } from './MessageComposer'

const fetchMock = vi.fn()

describe('MessageComposer', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        fetchMock.mockResolvedValue(new Response(JSON.stringify({ id: 'm1' }), { status: 201 }))
        vi.stubGlobal('fetch', fetchMock)
    })
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('sends on Enter, keeps Shift+Enter as a line break and clears the field', async () => {
        const onCancel = vi.fn()
        renderWithProviders(<MessageComposer conversationId="c1" myId="me" onCancel={onCancel} />)
        const field = screen.getByLabelText('Сообщение')
        expect(screen.getByRole('button', { name: 'Отправить' })).toBeDisabled()

        fireEvent.change(field, { target: { value: '  привет  ' } })
        fireEvent.keyDown(field, { key: 'Enter', shiftKey: true })
        expect(fetchMock).not.toHaveBeenCalled()

        fireEvent.keyDown(field, { key: 'Enter' })
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/messages')
        expect(JSON.parse(init.body as string)).toEqual({
            conversation_id: 'c1',
            content: 'привет',
            reply_to_message_id: null,
        })
        expect(field).toHaveValue('')
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
        fireEvent.change(screen.getByLabelText('Сообщение'), { target: { value: 'ответ' } })
        fireEvent.click(screen.getByRole('button', { name: 'Отправить' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        expect(JSON.parse(fetchMock.mock.calls[0]![1].body as string).reply_to_message_id).toBe(
            'm9',
        )
    })

    it('edits a message and saves only a changed text', async () => {
        const editing = { id: 'm5', content: 'старый текст' } as Message
        renderWithProviders(
            <MessageComposer conversationId="c1" myId="me" editing={editing} onCancel={vi.fn()} />,
        )
        const field = screen.getByLabelText('Сообщение')
        expect(field).toHaveValue('старый текст')
        // курсор стоит в конце текста, а не в начале
        const input = field as HTMLTextAreaElement
        expect(input.selectionStart).toBe('старый текст'.length)
        expect(input.selectionEnd).toBe('старый текст'.length)
        expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()

        fireEvent.change(field, { target: { value: 'новый текст' } })
        fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
        await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
        const [url, init] = fetchMock.mock.calls[0]!
        expect(url).toBe('/api/messages/m5')
        expect(init.method).toBe('PATCH')
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
            const field = screen.getByLabelText('Сообщение') as HTMLTextAreaElement
            fireEvent.change(field, { target: { value: 'приветмир' } })
            field.setSelectionRange(6, 6)

            const people = await open()
            const emoji = firstEmoji(people)
            fireEvent.click(within(people).getAllByRole('button')[0]!)
            expect(field).toHaveValue(`привет${emoji}мир`)
            // панель осталась открытой, курсор - сразу после эмодзи
            expect(screen.getByRole('dialog', { name: 'Эмодзи' })).toBeInTheDocument()
            await vi.waitFor(() => expect(field.selectionStart).toBe(6 + emoji.length))
            // сообщение с эмодзи отправляется как обычный текст
            expect(screen.getByRole('button', { name: 'Отправить' })).toBeEnabled()
        })

        it('replaces the selected text and closes with Esc or a click outside', async () => {
            fetchMock.mockResolvedValue(new Response(JSON.stringify({ favorite_emojis: [] })))
            renderWithProviders(
                <MessageComposer conversationId="c1" myId="me" onCancel={vi.fn()} />,
            )
            const field = screen.getByLabelText('Сообщение') as HTMLTextAreaElement
            fireEvent.change(field, { target: { value: 'abcdef' } })
            field.setSelectionRange(1, 4)
            const people = await open()
            const emoji = firstEmoji(people)
            fireEvent.click(within(people).getAllByRole('button')[0]!)
            expect(field).toHaveValue(`a${emoji}ef`)

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
})
