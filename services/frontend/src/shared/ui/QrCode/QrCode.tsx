import { useMemo } from 'react'

import { encodeQr } from './encode'
import styles from './QrCode.module.scss'

interface Props {
    value: string
    size?: number
    label?: string
}

const QUIET_ZONE = 4

// векторный QR-код с тихой зоной 4 модуля; фон всегда белый, чтобы сканер надёжно читал его в тёмной теме
export function QrCode({ value, size = 200, label = 'QR-код' }: Props) {
    const encoded = useMemo(() => {
        try {
            const matrix = encodeQr(value)
            const n = matrix.length
            const parts: string[] = []
            for (let r = 0; r < n; r++) {
                for (let c = 0; c < n; c++) {
                    if (matrix[r]![c]) {
                        parts.push(`M${c + QUIET_ZONE},${r + QUIET_ZONE}h1v1h-1z`)
                    }
                }
            }
            return { total: n + QUIET_ZONE * 2, path: parts.join('') }
        } catch {
            return null
        }
    }, [value])

    if (!encoded) {
        return (
            <p className={styles.fallback} role="status">
                QR-код недоступен — введите ключ вручную
            </p>
        )
    }

    return (
        <svg
            role="img"
            aria-label={label}
            width={size}
            height={size}
            viewBox={`0 0 ${encoded.total} ${encoded.total}`}
            shapeRendering="crispEdges"
            className={styles.qr}
        >
            <rect width={encoded.total} height={encoded.total} fill="#ffffff" />
            <path d={encoded.path} fill="#111111" />
        </svg>
    )
}
