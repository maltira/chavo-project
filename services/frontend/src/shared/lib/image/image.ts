// фото перед загрузкой: проверка файла, чтение с учётом поворота из EXIF, кроп в квадрат и кодирование в JPEG.
// это удобство, а не защита: сервер всё равно проверяет и перекодирует файл сам

// что показывает выбор файла; HEIC и SVG браузер в canvas не отрисует (а SVG сервер и не примет)
export const IMAGE_ACCEPT = 'image/jpeg,image/png,image/gif,image/webp'
const ACCEPTED_TYPES = IMAGE_ACCEPT.split(',')

// больше нет смысла: на сервер уходит кроп 512×512, а исходник только читается в браузере
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024

// сторона квадрата, который уходит на сервер (сервер уменьшит до своего размера)
export const AVATAR_UPLOAD_SIZE = 512

// масштаб в окне кропа: 1 - фото целиком закрывает круг, 4 - максимальное приближение
export const MIN_ZOOM = 1
export const MAX_ZOOM = 4

// текст ошибки для выбранного файла или null, если его можно открыть
export function checkImageFile(file: File): string | null {
    if (!ACCEPTED_TYPES.includes(file.type)) {
        return 'Выберите фото в формате JPEG, PNG, GIF или WebP'
    }
    if (file.size > MAX_IMAGE_BYTES) {
        return 'Фото больше 10 МБ'
    }
    return null
}

export interface LoadedImage {
    source: CanvasImageSource
    width: number
    height: number
}

// декодирует файл; поворот из EXIF применяется сразу, поэтому фото с телефона не ложится набок
export async function loadImage(file: File): Promise<LoadedImage> {
    const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
    return { source: bitmap, width: bitmap.width, height: bitmap.height }
}

// положение фото в квадратном окне кропа: zoom и смещение центра фото от центра окна (px окна)
export interface CropState {
    zoom: number
    x: number
    y: number
}

// масштаб, при котором короткая сторона фото равна стороне окна (zoom = 1)
export function coverScale(width: number, height: number, viewport: number): number {
    return viewport / Math.min(width, height)
}

// смещение так, чтобы фото всегда закрывало окно целиком, без пустых полос
export function clampCrop(
    crop: CropState,
    width: number,
    height: number,
    viewport: number,
): CropState {
    const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, crop.zoom))
    const scale = coverScale(width, height, viewport) * zoom
    const maxX = Math.max(0, (width * scale - viewport) / 2)
    const maxY = Math.max(0, (height * scale - viewport) / 2)
    // || 0 убирает -0, когда смещать некуда
    return {
        zoom,
        x: Math.min(maxX, Math.max(-maxX, crop.x)) || 0,
        y: Math.min(maxY, Math.max(-maxY, crop.y)) || 0,
    }
}

// какой квадрат исходного фото виден в окне (в пикселях исходника)
export function sourceRect(crop: CropState, width: number, height: number, viewport: number) {
    const scale = coverScale(width, height, viewport) * crop.zoom
    const size = viewport / scale
    return {
        sx: width / 2 - (viewport / 2 + crop.x) / scale,
        sy: height / 2 - (viewport / 2 + crop.y) / scale,
        size,
    }
}

// вырезает квадрат и кодирует его в JPEG; прозрачность - на белый, как и на сервере
export function encodeSquare(
    image: LoadedImage,
    rect: { sx: number; sy: number; size: number },
    size = AVATAR_UPLOAD_SIZE,
): Promise<Blob> {
    const canvas = document.createElement('canvas')
    canvas.width = size
    canvas.height = size
    const ctx = canvas.getContext('2d')
    if (!ctx) {
        return Promise.reject(new Error('canvas is not available'))
    }
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, size, size)
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(image.source, rect.sx, rect.sy, rect.size, rect.size, 0, 0, size, size)
    return new Promise((resolve, reject) =>
        canvas.toBlob(
            (blob) => (blob ? resolve(blob) : reject(new Error('encoding failed'))),
            'image/jpeg',
            0.9,
        ),
    )
}
