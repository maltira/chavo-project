import { useEffect, useRef } from 'react'
import type { RefObject } from 'react'

import styles from './RichText.module.scss'

type RGB = [number, number, number]

interface Particle {
    x: number
    y: number
    vx: number
    vy: number
    r: number
    age: number
    life: number
    peak: number
    accent: boolean
    star: boolean
}

export interface RevealOrigin {
    x: number
    y: number
}

interface Props {
    hostRef: RefObject<HTMLSpanElement | null>
    revealed: boolean
    origin: RevealOrigin | null
}

const PAD = 6
const DENSITY = 0.3
const MIN_COUNT = 50
const MAX_COUNT = 460
const SPEED = 12
const LIFE_MIN = 0.7
const LIFE_MAX = 1.9
const BURST_SPEED = 72
const REVEAL_SPEED = 3.1

const DEFAULT_TEXT_RGB: RGB = [238, 241, 247]
const DEFAULT_ACCENT_RGB: RGB = [124, 92, 255]

// общий rAF-цикл для всех видимых спойлеров на странице: останавливается, когда анимировать нечего
const tickers = new Set<(dt: number) => void>()
let rafId: number | null = null
let prevTime = 0

function stepAll(now: number) {
    if (tickers.size === 0) {
        rafId = null
        return
    }
    const dt = prevTime === 0 ? 0.016 : Math.min(0.05, (now - prevTime) / 1000)
    prevTime = now
    for (const tick of tickers) {
        tick(dt)
    }
    if (tickers.size > 0) {
        rafId = requestAnimationFrame(stepAll)
    } else {
        rafId = null
    }
}

function subscribeTicker(tick: (dt: number) => void) {
    tickers.add(tick)
    if (rafId === null && typeof requestAnimationFrame === 'function') {
        prevTime = performance.now()
        rafId = requestAnimationFrame(stepAll)
    }
}

function unsubscribeTicker(tick: (dt: number) => void) {
    tickers.delete(tick)
    if (tickers.size === 0 && rafId !== null && typeof cancelAnimationFrame === 'function') {
        cancelAnimationFrame(rafId)
        rafId = null
    }
}

function parseColor(raw: string, fallback: RGB): RGB {
    const trimmed = raw.trim()
    const rgbMatch = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/i.exec(trimmed)
    if (rgbMatch) {
        return [
            Math.round(Number(rgbMatch[1])),
            Math.round(Number(rgbMatch[2])),
            Math.round(Number(rgbMatch[3])),
        ]
    }
    const hex = trimmed.startsWith('#') ? trimmed.slice(1) : ''
    if (hex.length === 3) {
        return [
            parseInt(hex[0]! + hex[0]!, 16),
            parseInt(hex[1]! + hex[1]!, 16),
            parseInt(hex[2]! + hex[2]!, 16),
        ]
    }
    if (hex.length === 6) {
        return [
            parseInt(hex.slice(0, 2), 16),
            parseInt(hex.slice(2, 4), 16),
            parseInt(hex.slice(4, 6), 16),
        ]
    }
    return fallback
}

function readPalette(host: HTMLElement): { text: RGB; accent: RGB; isLight: boolean } {
    const cs = getComputedStyle(host)
    const text = parseColor(cs.color, DEFAULT_TEXT_RGB)
    const accent = parseColor(cs.getPropertyValue('--color-accent'), DEFAULT_ACCENT_RGB)
    // тёмный текст означает светлую тему
    const lum = 0.299 * text[0] + 0.587 * text[1] + 0.114 * text[2]
    return { text, accent, isLight: lum < 128 }
}

function mixRGB(a: RGB, b: RGB, t: number): RGB {
    return [
        Math.round(a[0] + (b[0] - a[0]) * t),
        Math.round(a[1] + (b[1] - a[1]) * t),
        Math.round(a[2] + (b[2] - a[2]) * t),
    ]
}

function rgba(rgb: RGB, alpha: number): string {
    return `rgba(${rgb[0]},${rgb[1]},${rgb[2]},${alpha})`
}

function spawnParticle(w: number, h: number, randomAge: boolean): Particle {
    const angle = Math.random() * Math.PI * 2
    const spd = (0.35 + Math.random() * 0.65) * SPEED
    const life = LIFE_MIN + Math.random() * (LIFE_MAX - LIFE_MIN)
    return {
        x: PAD + Math.random() * w,
        y: PAD + Math.random() * h,
        vx: Math.cos(angle) * spd,
        vy: Math.sin(angle) * spd,
        r: 0.5 + Math.random() * 0.5,
        age: randomAge ? Math.random() * life : 0,
        life,
        peak: 0.55 + Math.random() * 0.45,
        accent: Math.random() < 0.25,
        star: Math.random() < 0.11,
    }
}

function drawStar(ctx: CanvasRenderingContext2D, x: number, y: number, size: number, color: string) {
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.moveTo(x, y - size)
    ctx.quadraticCurveTo(x, y, x + size, y)
    ctx.quadraticCurveTo(x, y, x, y + size)
    ctx.quadraticCurveTo(x, y, x - size, y)
    ctx.quadraticCurveTo(x, y, x, y - size)
    ctx.fill()
}

// canvas-слой спойлера «Алмазная крошка»: мелкие мерцающие частицы и 4-лучевые искорки,
// которые разлетаются волной от точки клика при раскрытии
export function SpoilerParticles({ hostRef, revealed, origin }: Props) {
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const stateRef = useRef<'hidden' | 'revealing' | 'revealed'>(revealed ? 'revealed' : 'hidden')
    const originRef = useRef<RevealOrigin | null>(origin)
    const triggerRevealRef = useRef<(() => void) | null>(null)

    useEffect(() => {
        originRef.current = origin
        if (revealed && stateRef.current === 'hidden') {
            triggerRevealRef.current?.()
        }
    }, [origin, revealed])

    useEffect(() => {
        const host = hostRef.current
        const canvas = canvasRef.current
        if (!host || !canvas) {
            return
        }
        let ctx: CanvasRenderingContext2D | null = null
        try {
            ctx = canvas.getContext('2d')
        } catch {
            return
        }
        if (!ctx) {
            return
        }

        const reducedMotion =
            typeof window !== 'undefined' &&
            typeof window.matchMedia === 'function' &&
            window.matchMedia('(prefers-reduced-motion: reduce)').matches

        const dpr = Math.min(window.devicePixelRatio || 1, 2)
        let w = 0
        let h = 0
        let particles: Particle[] = []
        let revT = 0
        let clickX = PAD
        let clickY = PAD
        let visible = true
        let palette = readPalette(host)

        const seed = () => {
            const count = Math.max(MIN_COUNT, Math.min(MAX_COUNT, Math.round(w * h * DENSITY)))
            particles = Array.from({ length: count }, () => spawnParticle(w, h, true))
        }

        const drawFrame = (dt: number) => {
            if (!ctx) {
                return
            }
            ctx.clearRect(0, 0, w + PAD * 2, h + PAD * 2)
            if (stateRef.current === 'revealed') {
                unsubscribeTicker(drawFrame)
                return
            }

            let globalFade = 1
            if (stateRef.current === 'revealing') {
                revT += dt * REVEAL_SPEED
                globalFade = Math.max(0, 1 - revT)
                if (globalFade <= 0) {
                    stateRef.current = 'revealed'
                    unsubscribeTicker(drawFrame)
                    return
                }
            }

            const maxD = Math.hypot(w, h) + 1
            const accentTint = palette.isLight
                ? palette.accent
                : mixRGB(palette.accent, [255, 255, 255], 0.25)
            const baseAlpha = palette.isLight ? 0.78 : 0.9

            for (let i = 0; i < particles.length; i++) {
                const p = particles[i]!
                if (stateRef.current === 'hidden' && dt > 0) {
                    p.age += dt
                    if (p.age >= p.life) {
                        particles[i] = spawnParticle(w, h, false)
                        continue
                    }
                    p.x += p.vx * dt
                    p.y += p.vy * dt
                    if (p.x < PAD || p.x > PAD + w || p.y < PAD || p.y > PAD + h) {
                        particles[i] = spawnParticle(w, h, false)
                        continue
                    }
                } else if (stateRef.current === 'revealing') {
                    p.x += p.vx * dt
                    p.y += p.vy * dt
                    p.vx *= 0.94
                    p.vy *= 0.94
                }

                const lifeAlpha =
                    stateRef.current === 'hidden' ? Math.sin((p.age / p.life) * Math.PI) : 1

                let waveFade = globalFade
                if (stateRef.current === 'revealing') {
                    const distRatio = Math.hypot(p.x - clickX, p.y - clickY) / maxD
                    waveFade = Math.max(0, Math.min(1, 1 - (revT * 1.25 - distRatio * 0.35)))
                }

                const alpha = lifeAlpha * p.peak * baseAlpha * waveFade
                if (alpha <= 0.02) {
                    continue
                }
                const color = p.accent ? rgba(accentTint, alpha) : rgba(palette.text, alpha * 0.92)
                if (p.star) {
                    drawStar(ctx, p.x, p.y, p.r * 2.8, color)
                } else {
                    ctx.fillStyle = color
                    ctx.beginPath()
                    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
                    ctx.fill()
                }
            }
        }

        const syncRunning = () => {
            if (reducedMotion || stateRef.current === 'revealed' || !visible) {
                unsubscribeTicker(drawFrame)
                return
            }
            subscribeTicker(drawFrame)
        }

        const resize = () => {
            const rect = host.getBoundingClientRect()
            const nw = Math.max(12, Math.round(rect.width))
            const nh = Math.max(12, Math.round(rect.height))
            if (nw === w && nh === h && particles.length > 0) {
                return
            }
            w = nw
            h = nh
            canvas.width = Math.round((w + PAD * 2) * dpr)
            canvas.height = Math.round((h + PAD * 2) * dpr)
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
            seed()
            if (reducedMotion && stateRef.current === 'hidden') {
                drawFrame(0)
            }
        }

        triggerRevealRef.current = () => {
            if (reducedMotion) {
                stateRef.current = 'revealed'
                ctx.clearRect(0, 0, w + PAD * 2, h + PAD * 2)
                unsubscribeTicker(drawFrame)
                return
            }
            const o = originRef.current
            clickX = (o ? o.x : w / 2) + PAD
            clickY = (o ? o.y : h / 2) + PAD
            for (const p of particles) {
                const dx = p.x - clickX
                const dy = p.y - clickY
                const dist = Math.hypot(dx, dy) + 6
                const push = BURST_SPEED * (0.65 + Math.random() * 0.7)
                p.vx = (dx / dist) * push + (Math.random() - 0.5) * 16
                p.vy = (dy / dist) * push + (Math.random() - 0.5) * 16
            }
            stateRef.current = 'revealing'
            revT = 0
            subscribeTicker(drawFrame)
        }

        resize()
        syncRunning()

        const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : null
        ro?.observe(host)

        const io =
            typeof IntersectionObserver === 'function'
                ? new IntersectionObserver((entries) => {
                      visible = entries[0]?.isIntersecting ?? true
                      syncRunning()
                  })
                : null
        io?.observe(host)

        const mo =
            typeof MutationObserver === 'function'
                ? new MutationObserver(() => {
                      palette = readPalette(host)
                      if (reducedMotion && stateRef.current === 'hidden') {
                          drawFrame(0)
                      }
                  })
                : null
        mo?.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ['style', 'class', 'data-theme', 'data-accent'],
        })

        return () => {
            triggerRevealRef.current = null
            unsubscribeTicker(drawFrame)
            ro?.disconnect()
            io?.disconnect()
            mo?.disconnect()
        }
    }, [hostRef])

    return <canvas ref={canvasRef} aria-hidden="true" className={styles.spoilerCanvas} />
}
