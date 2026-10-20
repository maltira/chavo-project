import { setAccessToken } from '@/shared/api'

import { currentSessionId } from './sessions'

// токен в формате JWT: заголовок.полезная нагрузка.подпись (base64url)
const token = (payload: object) =>
    `x.${btoa(JSON.stringify(payload)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')}.y`

describe('currentSessionId', () => {
    afterEach(() => setAccessToken(null))

    it('reads sid from the access token', () => {
        setAccessToken(token({ sub: 'u1', sid: 's-42' }))
        expect(currentSessionId()).toBe('s-42')
    })

    it('is null without a token or with a broken one', () => {
        expect(currentSessionId()).toBeNull()
        setAccessToken('broken')
        expect(currentSessionId()).toBeNull()
        setAccessToken('x.%%%.y')
        expect(currentSessionId()).toBeNull()
    })
})
