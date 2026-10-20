import { fireEvent, screen } from '@testing-library/react'

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
})
