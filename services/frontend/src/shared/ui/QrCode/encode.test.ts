import { encodeQr, formatBits, versionBits } from './encode'

describe('QR code encoder', () => {
    it('computes ISO/IEC 18004 format bits for EC level M', () => {
        // EC M = 00, mask 0 -> 0x0000 ^ 0x5412 = 0x5412
        expect(formatBits(0)).toBe(0x5412)
        // EC M = 00, mask 1 -> 0x5125
        expect(formatBits(1)).toBe(0x5125)
    })

    it('computes ISO/IEC 18004 version info bits for v7', () => {
        expect(versionBits(7)).toBe(0x07c94)
    })

    it('encodes short text into v1 (21x21) with valid finder and timing patterns', () => {
        const m = encodeQr('HELLO')
        expect(m).toHaveLength(21)
        expect(m[0]).toHaveLength(21)

        // верхний левый finder pattern 7x7: углы тёмные, внутренний ободок светлый, центр 3x3 тёмный
        expect(m[0]![0]).toBe(true)
        expect(m[0]![6]).toBe(true)
        expect(m[1]![1]).toBe(false)
        expect(m[3]![3]).toBe(true)
        // тёмный модуль на (4*v + 9, 8) = (13, 8)
        expect(m[13]![8]).toBe(true)
        // полоса синхронизации на 6-й строке
        expect(m[6]![8]).toBe(true)
        expect(m[6]![9]).toBe(false)
    })

    it('encodes a typical otpauth TOTP URI into a square matrix', () => {
        const uri =
            'otpauth://totp/Chavo:user%40example.com?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=Chavo&algorithm=SHA1&digits=6&period=30'
        const m = encodeQr(uri)
        expect(m.length).toBeGreaterThanOrEqual(45)
        expect(m.every((row) => row.length === m.length)).toBe(true)
        // тёмный модуль всегда на (size - 8, 8)
        expect(m[m.length - 8]![8]).toBe(true)
    })

    it('encodes long otpauth URIs with maximum-length emails (237..360 bytes)', () => {
        const longEmail = `${'a'.repeat(64)}@${'sub.'.repeat(40)}example.com`
        const uri = `otpauth://totp/Chavo:${encodeURIComponent(longEmail)}?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=Chavo&algorithm=SHA1&digits=6&period=30`
        expect(new TextEncoder().encode(uri).length).toBeGreaterThan(237)
        const m = encodeQr(uri)
        expect(m.length).toBeGreaterThanOrEqual(61)
        expect(m.every((row) => row.length === m.length)).toBe(true)
        expect(m[m.length - 8]![8]).toBe(true)
    })
})
