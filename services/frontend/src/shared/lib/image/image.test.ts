import {
    AVATAR_MIN_SIDE,
    checkImageFile,
    clampCrop,
    coverScale,
    isTooSmall,
    MAX_IMAGE_BYTES,
    maxZoomFor,
    sourceRect,
} from './image'

const file = (type: string, size = 10) => new File([new Uint8Array(size)], 'x', { type })

describe('checkImageFile', () => {
    it('accepts raster photos and rejects anything else', () => {
        for (const type of ['image/jpeg', 'image/png', 'image/gif', 'image/webp']) {
            expect(checkImageFile(file(type))).toBeNull()
        }
        for (const type of [
            'image/svg+xml',
            'image/heic',
            'application/x-msdownload',
            'text/html',
            '',
        ]) {
            expect(checkImageFile(file(type))).toMatch(/JPEG, PNG, GIF или WebP/)
        }
        expect(checkImageFile(file('image/jpeg', MAX_IMAGE_BYTES + 1))).toMatch(/10 МБ/)
    })
})

describe('crop math', () => {
    // фото 400×200 в окне 100: короткая сторона закрывает окно
    const W = 400
    const H = 200
    const V = 100

    it('covers the window with the short side at zoom 1', () => {
        expect(coverScale(W, H, V)).toBe(0.5)
        // по центру видна середина фото
        expect(sourceRect({ zoom: 1, x: 0, y: 0 }, W, H, V)).toEqual({ sx: 100, sy: 0, size: 200 })
    })

    it('never leaves an empty strip when dragged too far', () => {
        // при zoom 1 фото шире окна на 100px (200 - 100): двигать можно на 50 в каждую сторону, по вертикали - нельзя
        expect(clampCrop({ zoom: 1, x: 500, y: 40 }, W, H, V)).toEqual({ zoom: 1, x: 50, y: 0 })
        expect(clampCrop({ zoom: 1, x: -500, y: -40 }, W, H, V)).toEqual({ zoom: 1, x: -50, y: 0 })
        const left = sourceRect({ zoom: 1, x: 50, y: 0 }, W, H, V)
        expect(left.sx).toBe(0)
    })

    it('keeps zoom within bounds and shows less of the photo when zoomed', () => {
        // большое фото: приближать можно до общего предела 4
        expect(clampCrop({ zoom: 10, x: 0, y: 0 }, 4000, 3000, V).zoom).toBe(4)
        expect(clampCrop({ zoom: 0.1, x: 0, y: 0 }, W, H, V).zoom).toBe(1)
        expect(sourceRect({ zoom: 2, x: 0, y: 0 }, W, H, V)).toEqual({ sx: 150, sy: 50, size: 100 })
    })
})

describe('minimum photo size', () => {
    it('rejects a photo shorter than 512 px on any side', () => {
        expect(AVATAR_MIN_SIDE).toBe(512)
        expect(isTooSmall(511, 2000)).toBe(true)
        expect(isTooSmall(2000, 511)).toBe(true)
        expect(isTooSmall(512, 512)).toBe(false)
    })

    it('stops zooming where the crop would get smaller than 512 real pixels', () => {
        // короткая сторона 1024: при зуме 2 кроп ровно 512, дальше сервер его не примет
        expect(maxZoomFor(2048, 1024)).toBe(2)
        expect(maxZoomFor(512, 512)).toBe(1)
        // очень большое фото - общий предел
        expect(maxZoomFor(8000, 6000)).toBe(4)
        const crop = clampCrop({ zoom: 4, x: 0, y: 0 }, 2048, 1024, 100)
        expect(crop.zoom).toBe(2)
        expect(sourceRect(crop, 2048, 1024, 100).size).toBe(512)
    })
})
