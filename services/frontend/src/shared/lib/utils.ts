import { type ClassValue, clsx } from 'clsx'

export function cn(...inputs: ClassValue[]) {
    return clsx(inputs)
}

// русское склонение по числу: plural(5, ['участник', 'участника', 'участников']) -> 'участников'
export function plural(n: number, forms: [string, string, string]): string {
    const mod10 = n % 10
    const mod100 = n % 100
    if (mod10 === 1 && mod100 !== 11) {
        return forms[0]
    }
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
        return forms[1]
    }
    return forms[2]
}
