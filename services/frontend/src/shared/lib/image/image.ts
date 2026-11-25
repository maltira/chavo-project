// фото перед загрузкой: проверка файла, чтение с учётом поворота из EXIF, кроп в квадрат и кодирование в JPEG.
// это удобство, а не защита: сервер всё равно проверяет и перекодирует файл сам

// наименьшая сторона загружаемого фото: меньшее сервер не принимает (крупная версия была бы размытой)
export const AVATAR_MIN_SIDE = 512

// что показывает выбор файла; HEIC и SVG браузер в canvas не отрисует (а SVG сервер и не примет)
export const IMAGE_ACCEPT = 'image/jpeg,image/png,image/gif,image/webp'
const ACCEPTED_TYPES = IMAGE_ACCEPT.split(',')

// больше нет смысла: на сервер уходит кроп не больше 1024×1024, а исходник только читается в браузере
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024

// наибольшая сторона квадрата, который уходит на сервер: из него сервер делает маленькую аватарку и крупную
// для просмотра на весь экран; фото меньшего размера не растягивается
export const AVATAR_UPLOAD_SIZE = 1024
// наименьшая сторона кропа на выходе: сервер принимает не меньше AVATAR_MIN_SIDE
const AVATAR_MIN_SIZE = AVATAR_MIN_SIDE
// весь запрос с фото на сервере ограничен 1 МиБ: кодируем с запасом на служебные части multipart
const UPLOAD_BYTES_LIMIT = 900 * 1024
// качества JPEG по убыванию: берём первое, при котором кроп укладывается в предел
const JPEG_QUALITIES = [0.9, 0.8, 0.7, 0.55, 0.4]

// масштаб в окне кропа: 1 - фото целиком закрывает круг, 4 - максимальное приближение
export const MIN_ZOOM = 1
export const MAX_ZOOM = 4

// фото слишком маленькое для аватарки
export function isTooSmall(width: number, height: number): boolean {
    return Math.min(width, height) < AVATAR_MIN_SIDE
}

// наибольший зум: кроп не должен стать меньше AVATAR_MIN_SIDE реальных пикселей фото, иначе сервер его
// не примет; короткая сторона фото при зуме z даёт кроп в min(w, h) / z пикселей
export function maxZoomFor(width: number, height: number): number {
    return Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, Math.min(width, height) / AVATAR_MIN_SIDE))
}

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
    const zoom = Math.min(maxZoomFor(width, height), Math.max(MIN_ZOOM, crop.zoom))
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

// вырезает квадрат и кодирует его в JPEG; прозрачность - на белый, как и на сервере.
// Сторона - размер кропа в пикселях исходного фото, но не больше AVATAR_UPLOAD_SIZE и не меньше
// AVATAR_MIN_SIZE (size можно задать явно); плотное фото сжимается сильнее, чтобы уложиться в предел запроса
export async function encodeSquare(
    image: LoadedImage,
    rect: { sx: number; sy: number; size: number },
    size = Math.min(AVATAR_UPLOAD_SIZE, Math.max(AVATAR_MIN_SIZE, Math.round(rect.size))),
): Promise<Blob> {
    const canvas = document.createElement('canvas')
    canvas.width = size
    canvas.height = size
    const ctx = canvas.getContext('2d')
    if (!ctx) {
        throw new Error('canvas is not available')
    }
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, size, size)
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(image.source, rect.sx, rect.sy, rect.size, rect.size, 0, 0, size, size)

    let blob: Blob | null = null
    for (const quality of JPEG_QUALITIES) {
        blob = await new Promise<Blob | null>((resolve) =>
            canvas.toBlob(resolve, 'image/jpeg', quality),
        )
        if (!blob) {
            throw new Error('encoding failed')
        }
        if (blob.size <= UPLOAD_BYTES_LIMIT) {
            break
        }
    }
    return blob!
}
