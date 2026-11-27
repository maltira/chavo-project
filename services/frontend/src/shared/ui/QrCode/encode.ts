// генератор QR-кодов (ISO/IEC 18004, байтовый режим, уровень коррекции M, версии 1-10)
// без сторонних зависимостей - используется для вывода otpauth:// URI при настройке TOTP

interface VersionSpec {
    version: number
    size: number
    ecPerBlock: number
    groups: [count: number, dataCodewords: number][]
    align: number[]
}

// таблица версий 1..16 для уровня коррекции M (ISO/IEC 18004, таблицы 9 и E.1; до 450 байт)
const VERSIONS: VersionSpec[] = [
    { version: 1, size: 21, ecPerBlock: 10, groups: [[1, 16]], align: [] },
    { version: 2, size: 25, ecPerBlock: 16, groups: [[1, 28]], align: [6, 18] },
    { version: 3, size: 29, ecPerBlock: 26, groups: [[1, 44]], align: [6, 22] },
    { version: 4, size: 33, ecPerBlock: 18, groups: [[2, 32]], align: [6, 26] },
    { version: 5, size: 37, ecPerBlock: 24, groups: [[2, 43]], align: [6, 30] },
    { version: 6, size: 41, ecPerBlock: 16, groups: [[4, 27]], align: [6, 34] },
    { version: 7, size: 45, ecPerBlock: 18, groups: [[4, 31]], align: [6, 22, 38] },
    {
        version: 8,
        size: 49,
        ecPerBlock: 22,
        groups: [
            [2, 38],
            [2, 39],
        ],
        align: [6, 24, 42],
    },
    {
        version: 9,
        size: 53,
        ecPerBlock: 22,
        groups: [
            [3, 36],
            [2, 37],
        ],
        align: [6, 26, 46],
    },
    {
        version: 10,
        size: 57,
        ecPerBlock: 26,
        groups: [
            [4, 43],
            [1, 44],
        ],
        align: [6, 28, 50],
    },
    {
        version: 11,
        size: 61,
        ecPerBlock: 30,
        groups: [
            [1, 50],
            [4, 51],
        ],
        align: [6, 30, 54],
    },
    {
        version: 12,
        size: 65,
        ecPerBlock: 22,
        groups: [
            [6, 36],
            [2, 37],
        ],
        align: [6, 32, 58],
    },
    {
        version: 13,
        size: 69,
        ecPerBlock: 22,
        groups: [
            [8, 37],
            [1, 38],
        ],
        align: [6, 34, 62],
    },
    {
        version: 14,
        size: 73,
        ecPerBlock: 24,
        groups: [
            [4, 40],
            [5, 41],
        ],
        align: [6, 26, 46, 66],
    },
    {
        version: 15,
        size: 77,
        ecPerBlock: 24,
        groups: [
            [5, 41],
            [5, 42],
        ],
        align: [6, 26, 48, 70],
    },
    {
        version: 16,
        size: 81,
        ecPerBlock: 28,
        groups: [
            [7, 45],
            [3, 46],
        ],
        align: [6, 26, 50, 74],
    },
]

// таблицы GF(256) с примитивным полиномом x^8 + x^4 + x^3 + x^2 + 1 (0x11d)
const GF_EXP = new Uint8Array(512)
const GF_LOG = new Uint8Array(256)
{
    let x = 1
    for (let i = 0; i < 255; i++) {
        GF_EXP[i] = x
        GF_LOG[x] = i
        x <<= 1
        if (x & 0x100) {
            x ^= 0x11d
        }
    }
    for (let i = 255; i < 512; i++) {
        GF_EXP[i] = GF_EXP[i - 255]!
    }
}

function gfMul(a: number, b: number): number {
    if (a === 0 || b === 0) {
        return 0
    }
    return GF_EXP[GF_LOG[a]! + GF_LOG[b]!]!
}

function rsGenerator(degree: number): Uint8Array {
    let poly = new Uint8Array([1])
    for (let i = 0; i < degree; i++) {
        const next = new Uint8Array(poly.length + 1)
        const factor = GF_EXP[i]!
        for (let j = 0; j < poly.length; j++) {
            next[j] = (next[j]! ^ poly[j]!) & 0xff
            next[j + 1] = (next[j + 1]! ^ gfMul(poly[j]!, factor)) & 0xff
        }
        poly = next
    }
    return poly
}

function rsRemainder(data: Uint8Array, ecCount: number): Uint8Array {
    const gen = rsGenerator(ecCount)
    const rem = new Uint8Array(ecCount)
    for (let i = 0; i < data.length; i++) {
        const factor = data[i]! ^ rem[0]!
        rem.copyWithin(0, 1)
        rem[ecCount - 1] = 0
        if (factor !== 0) {
            for (let j = 0; j < ecCount; j++) {
                rem[j] = (rem[j]! ^ gfMul(gen[j + 1]!, factor)) & 0xff
            }
        }
    }
    return rem
}

// 15-битная строка формата: EC M = 0b00, маска 0..7, BCH(15,5) по полиному 0x537, XOR 0x5412
export function formatBits(mask: number): number {
    const data = mask & 0x07 // старшие 2 бита для EC level M равны 00
    let rem = data << 10
    for (let i = 4; i >= 0; i--) {
        if (rem & (1 << (i + 10))) {
            rem ^= 0x537 << i
        }
    }
    return ((data << 10) | rem) ^ 0x5412
}

// 18-битная информация о версии для v >= 7: Golay(18,6) по полиному 0x1f25
export function versionBits(version: number): number {
    let rem = version << 12
    for (let i = 5; i >= 0; i--) {
        if (rem & (1 << (i + 12))) {
            rem ^= 0x1f25 << i
        }
    }
    return (version << 12) | rem
}

function totalDataCodewords(spec: VersionSpec): number {
    let sum = 0
    for (const [count, words] of spec.groups) {
        sum += count * words
    }
    return sum
}

function pickVersion(byteLength: number): VersionSpec {
    for (const spec of VERSIONS) {
        const countBits = spec.version >= 10 ? 16 : 8
        const capacityBits = totalDataCodewords(spec) * 8
        if (4 + countBits + byteLength * 8 <= capacityBits) {
            return spec
        }
    }
    throw new Error(`QR payload too long (${byteLength} bytes)`)
}

function buildCodewords(bytes: Uint8Array, spec: VersionSpec): Uint8Array {
    const dataCap = totalDataCodewords(spec)
    const bits: number[] = []
    const pushBits = (val: number, len: number) => {
        for (let i = len - 1; i >= 0; i--) {
            bits.push((val >> i) & 1)
        }
    }

    // байтовый режим (0100) + длина + полезная нагрузка
    pushBits(0b0100, 4)
    pushBits(bytes.length, spec.version >= 10 ? 16 : 8)
    for (const b of bytes) {
        pushBits(b, 8)
    }

    // терминатор (до 4 нулей) и выравнивание до границы байта
    const maxBits = dataCap * 8
    const term = Math.min(4, maxBits - bits.length)
    for (let i = 0; i < term; i++) {
        bits.push(0)
    }
    while (bits.length % 8 !== 0) {
        bits.push(0)
    }

    const data = new Uint8Array(dataCap)
    let byteIdx = 0
    for (let i = 0; i < bits.length; i += 8) {
        let v = 0
        for (let j = 0; j < 8; j++) {
            v = (v << 1) | bits[i + j]!
        }
        data[byteIdx++] = v
    }
    // добивка чередующимися байтами 0xEC и 0x11
    let pad = 0
    while (byteIdx < dataCap) {
        data[byteIdx++] = pad % 2 === 0 ? 0xec : 0x11
        pad++
    }

    // разбиение на блоки, вычисление RS и чередование
    const dataBlocks: Uint8Array[] = []
    const ecBlocks: Uint8Array[] = []
    let offset = 0
    let maxDataLen = 0
    for (const [count, words] of spec.groups) {
        for (let i = 0; i < count; i++) {
            const block = data.slice(offset, offset + words)
            offset += words
            dataBlocks.push(block)
            ecBlocks.push(rsRemainder(block, spec.ecPerBlock))
            if (words > maxDataLen) {
                maxDataLen = words
            }
        }
    }

    const out: number[] = []
    for (let i = 0; i < maxDataLen; i++) {
        for (const block of dataBlocks) {
            if (i < block.length) {
                out.push(block[i]!)
            }
        }
    }
    for (let i = 0; i < spec.ecPerBlock; i++) {
        for (const ec of ecBlocks) {
            out.push(ec[i]!)
        }
    }
    return new Uint8Array(out)
}

function maskBit(mask: number, r: number, c: number): boolean {
    switch (mask) {
        case 0:
            return (r + c) % 2 === 0
        case 1:
            return r % 2 === 0
        case 2:
            return c % 3 === 0
        case 3:
            return (r + c) % 3 === 0
        case 4:
            return (Math.floor(r / 2) + Math.floor(c / 3)) % 2 === 0
        case 5:
            return ((r * c) % 2) + ((r * c) % 3) === 0
        case 6:
            return (((r * c) % 2) + ((r * c) % 3)) % 2 === 0
        default:
            return (((r + c) % 2) + ((r * c) % 3)) % 2 === 0
    }
}

function placeFunctionPatterns(spec: VersionSpec): { modules: boolean[][]; reserved: boolean[][] } {
    const n = spec.size
    const modules = Array.from({ length: n }, () => Array<boolean>(n).fill(false))
    const reserved = Array.from({ length: n }, () => Array<boolean>(n).fill(false))

    const set = (r: number, c: number, dark: boolean) => {
        if (r < 0 || r >= n || c < 0 || c >= n) {
            return
        }
        modules[r]![c] = dark
        reserved[r]![c] = true
    }

    // три поисковых узора (finder patterns) с белой рамкой 1 модуль
    const placeFinder = (row: number, col: number) => {
        for (let dr = -1; dr <= 7; dr++) {
            for (let dc = -1; dc <= 7; dc++) {
                const inside = dr >= 0 && dr <= 6 && dc >= 0 && dc <= 6
                const border = dr === 0 || dr === 6 || dc === 0 || dc === 6
                const center = dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4
                set(row + dr, col + dc, inside && (border || center))
            }
        }
    }
    placeFinder(0, 0)
    placeFinder(0, n - 7)
    placeFinder(n - 7, 0)

    // полосы синхронизации (timing patterns)
    for (let i = 8; i < n - 8; i++) {
        set(6, i, i % 2 === 0)
        set(i, 6, i % 2 === 0)
    }

    // узоры выравнивания (alignment patterns)
    const centers = spec.align
    for (const r of centers) {
        for (const c of centers) {
            // пропускаем пересечения с тремя finder patterns
            if ((r <= 8 && c <= 8) || (r <= 8 && c >= n - 8) || (r >= n - 8 && c <= 8)) {
                continue
            }
            for (let dr = -2; dr <= 2; dr++) {
                for (let dc = -2; dc <= 2; dc++) {
                    const dark = Math.max(Math.abs(dr), Math.abs(dc)) !== 1
                    set(r + dr, c + dc, dark)
                }
            }
        }
    }

    // резервируем области формата и тёмный модуль
    for (let i = 0; i <= 8; i++) {
        if (i !== 6) {
            reserved[8]![i] = true
            reserved[i]![8] = true
        }
    }
    for (let i = n - 8; i < n; i++) {
        reserved[8]![i] = true
        reserved[i]![8] = true
    }
    set(n - 8, 8, true)

    // области информации о версии для v >= 7
    if (spec.version >= 7) {
        const vbits = versionBits(spec.version)
        for (let i = 0; i < 18; i++) {
            const dark = ((vbits >> i) & 1) === 1
            const row = Math.floor(i / 3)
            const col = n - 11 + (i % 3)
            set(row, col, dark)
            set(col, row, dark)
        }
    }

    return { modules, reserved }
}

function writeFormat(modules: boolean[][], mask: number) {
    const n = modules.length
    const bits = formatBits(mask)
    const bit = (pos: number) => ((bits >> pos) & 1) === 1

    // вокруг верхнего левого finder pattern
    for (let i = 0; i < 6; i++) {
        modules[8]![i] = bit(14 - i)
    }
    modules[8]![7] = bit(8)
    modules[8]![8] = bit(7)
    modules[7]![8] = bit(6)
    for (let i = 0; i < 6; i++) {
        modules[5 - i]![8] = bit(5 - i)
    }

    // нижний левый и верхний правый
    for (let i = 0; i < 7; i++) {
        modules[n - 1 - i]![8] = bit(14 - i)
    }
    for (let i = 0; i < 8; i++) {
        modules[8]![n - 8 + i] = bit(7 - i)
    }
}

function placeData(modules: boolean[][], reserved: boolean[][], codewords: Uint8Array) {
    const n = modules.length
    let bitIdx = 0
    const totalBits = codewords.length * 8
    let upward = true

    for (let right = n - 1; right >= 1; right -= 2) {
        if (right === 6) {
            right = 5 // пропускаем вертикальную полосу синхронизации
        }
        for (let step = 0; step < n; step++) {
            const r = upward ? n - 1 - step : step
            for (let dx = 0; dx < 2; dx++) {
                const c = right - dx
                if (reserved[r]![c]) {
                    continue
                }
                let dark = false
                if (bitIdx < totalBits) {
                    dark = ((codewords[bitIdx >> 3]! >> (7 - (bitIdx & 7))) & 1) === 1
                    bitIdx++
                }
                modules[r]![c] = dark
            }
        }
        upward = !upward
    }
}

function penaltyScore(m: boolean[][]): number {
    const n = m.length
    let score = 0

    // правило 1: серии из 5+ одинаковых модулей в строках и столбцах
    for (let r = 0; r < n; r++) {
        let run = 1
        for (let c = 1; c < n; c++) {
            if (m[r]![c] === m[r]![c - 1]) {
                run++
            } else {
                if (run >= 5) {
                    score += 3 + (run - 5)
                }
                run = 1
            }
        }
        if (run >= 5) {
            score += 3 + (run - 5)
        }
    }
    for (let c = 0; c < n; c++) {
        let run = 1
        for (let r = 1; r < n; r++) {
            if (m[r]![c] === m[r - 1]![c]) {
                run++
            } else {
                if (run >= 5) {
                    score += 3 + (run - 5)
                }
                run = 1
            }
        }
        if (run >= 5) {
            score += 3 + (run - 5)
        }
    }

    // правило 2: блоки 2x2 одного цвета
    for (let r = 0; r < n - 1; r++) {
        for (let c = 0; c < n - 1; c++) {
            const v = m[r]![c]
            if (v === m[r]![c + 1] && v === m[r + 1]![c] && v === m[r + 1]![c + 1]) {
                score += 3
            }
        }
    }

    // правило 3: ложные поисковые узоры (1:1:3:1:1 + 4 белых)
    const patA = [true, false, true, true, true, false, true, false, false, false, false]
    const patB = [false, false, false, false, true, false, true, true, true, false, true]
    const matchRow = (r: number, c: number, pat: boolean[]) => {
        for (let i = 0; i < 11; i++) {
            if (m[r]![c + i] !== pat[i]) {
                return false
            }
        }
        return true
    }
    const matchCol = (r: number, c: number, pat: boolean[]) => {
        for (let i = 0; i < 11; i++) {
            if (m[r + i]![c] !== pat[i]) {
                return false
            }
        }
        return true
    }
    for (let r = 0; r < n; r++) {
        for (let c = 0; c <= n - 11; c++) {
            if (matchRow(r, c, patA) || matchRow(r, c, patB)) {
                score += 40
            }
        }
    }
    for (let c = 0; c < n; c++) {
        for (let r = 0; r <= n - 11; r++) {
            if (matchCol(r, c, patA) || matchCol(r, c, patB)) {
                score += 40
            }
        }
    }

    // правило 4: баланс тёмных и светлых модулей
    let dark = 0
    for (let r = 0; r < n; r++) {
        for (let c = 0; c < n; c++) {
            if (m[r]![c]) {
                dark++
            }
        }
    }
    const percent = (dark * 100) / (n * n)
    const dev = Math.floor(Math.abs(percent - 50) / 5)
    score += dev * 10

    return score
}

// encodeQr возвращает квадратную матрицу QR-кода (true = тёмный модуль)
export function encodeQr(text: string): boolean[][] {
    const bytes = new TextEncoder().encode(text)
    const spec = pickVersion(bytes.length)
    const codewords = buildCodewords(bytes, spec)
    const { modules: base, reserved } = placeFunctionPatterns(spec)
    placeData(base, reserved, codewords)

    let best: boolean[][] | null = null
    let bestScore = Infinity

    for (let mask = 0; mask < 8; mask++) {
        const candidate = base.map((row, r) =>
            row.map((val, c) => (reserved[r]![c] ? val : maskBit(mask, r, c) ? !val : val)),
        )
        writeFormat(candidate, mask)
        const s = penaltyScore(candidate)
        if (s < bestScore) {
            bestScore = s
            best = candidate
        }
    }

    return best!
}
