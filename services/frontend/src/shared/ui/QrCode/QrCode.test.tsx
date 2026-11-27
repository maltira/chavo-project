import { render, screen } from '@testing-library/react'

import { QrCode } from './QrCode'

describe('QrCode', () => {
    it('renders SVG for long otpauth URIs', () => {
        const longEmail = `${'a'.repeat(64)}@${'sub.'.repeat(40)}example.com`
        const uri = `otpauth://totp/Chavo:${encodeURIComponent(longEmail)}?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=Chavo`
        render(<QrCode value={uri} label="QR-код TOTP" />)
        expect(screen.getByRole('img', { name: 'QR-код TOTP' })).toBeInTheDocument()
    })

    it('renders a safe fallback instead of crashing when payload exceeds max capacity', () => {
        render(<QrCode value={'x'.repeat(600)} label="QR-код TOTP" />)
        expect(screen.getByRole('status')).toHaveTextContent('QR-код недоступен')
    })
})
