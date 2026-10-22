import type { Message } from '../model/types'
import { quotePreview, systemParts } from './system'

const message = (content: string | null, is_deleted = false) =>
    ({ id: 'm1', content, is_deleted }) as Message

describe('quotePreview', () => {
    it('keeps a short text whole and in quotes', () => {
        expect(quotePreview(message('  Завтра   в 10  '))).toBe('«Завтра в 10»')
    })

    it('cuts a long text at a word boundary', () => {
        expect(
            quotePreview(message('Встречаемся завтра у главного входа, не опаздывайте пожалуйста')),
        ).toBe('«Встречаемся завтра у главного входа, не…»')
    })

    it('names a deleted or unknown message', () => {
        expect(quotePreview(message(null, true))).toBe('удалённое сообщение')
        expect(quotePreview(undefined)).toBe('…')
    })
})

describe('systemParts', () => {
    it('builds the pinned line with the quote', () => {
        const parts = systemParts(
            'pinned',
            { userId: 'u1', name: 'Давыд' },
            { userId: '', name: '' },
            { messageId: 'm1', text: '«Привет»' },
        )
        expect(parts).toEqual([
            { type: 'user', userId: 'u1', name: 'Давыд' },
            { type: 'text', text: ' закрепил сообщение ' },
            { type: 'quote', messageId: 'm1', text: '«Привет»' },
        ])
    })
})
