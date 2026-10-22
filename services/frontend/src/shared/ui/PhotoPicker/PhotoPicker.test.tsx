import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'

import { useNotifications } from '@/shared/lib'

import { PhotoPicker } from './PhotoPicker'

// в jsdom нет canvas и createImageBitmap: чтение и кодирование фото подменяются
const { loadImage, encodeSquare } = vi.hoisted(() => ({
    loadImage: vi.fn(),
    encodeSquare: vi.fn(),
}))
vi.mock('@/shared/lib', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/shared/lib')>()),
    loadImage,
    encodeSquare,
}))

const photo = (type = 'image/jpeg', name = 'me.jpg') =>
    new File([new Uint8Array(100)], name, { type })

function renderPicker(onPick = vi.fn()) {
    render(
        <PhotoPicker onPick={onPick}>
            {(open) => (
                <button type="button" onClick={open}>
                    Выбрать фото
                </button>
            )}
        </PhotoPicker>,
    )
    const input = screen.getByTestId('photo-input') as HTMLInputElement
    const choose = (file: File) => fireEvent.change(input, { target: { files: [file] } })
    return { onPick, input, choose }
}

describe('PhotoPicker', () => {
    beforeEach(() => {
        useNotifications.setState({ items: [] })
        loadImage.mockReset()
        encodeSquare.mockReset()
        loadImage.mockResolvedValue({ source: {}, width: 2000, height: 1500 })
        encodeSquare.mockResolvedValue(new Blob(['jpeg'], { type: 'image/jpeg' }))
    })

    it('opens the system file dialog from the trigger', () => {
        const { input } = renderPicker()
        const click = vi.spyOn(input, 'click')
        fireEvent.click(screen.getByRole('button', { name: 'Выбрать фото' }))
        expect(click).toHaveBeenCalled()
        expect(input.accept).toBe('image/jpeg,image/png,image/gif,image/webp')
    })

    it('refuses files that are not raster photos without opening the cropper', () => {
        const { choose } = renderPicker()
        choose(photo('application/x-msdownload', 'setup.exe'))
        choose(photo('image/svg+xml', 'logo.svg'))
        expect(screen.queryByRole('dialog', { name: 'Фото' })).toBeNull()
        expect(loadImage).not.toHaveBeenCalled()
        expect(useNotifications.getState().items[0]?.description).toMatch(/JPEG, PNG, GIF или WebP/)
    })

    it('crops the chosen photo and hands over the square', async () => {
        const { choose, onPick } = renderPicker()
        choose(photo())
        expect(screen.getByRole('dialog', { name: 'Фото' })).toBeInTheDocument()

        const save = await screen.findByRole('button', { name: 'Сохранить' })
        await waitFor(() => expect(save).toBeEnabled())
        // зум ползунком и сдвиг стрелкой меняют кадр, который уйдёт на кодирование
        fireEvent.change(screen.getByRole('slider', { name: 'Масштаб' }), {
            target: { value: '2' },
        })
        fireEvent.keyDown(screen.getByRole('group', { name: /Кадр фото/ }), { key: 'ArrowLeft' })
        await act(async () => fireEvent.click(save))

        expect(encodeSquare).toHaveBeenCalledTimes(1)
        const [, rect] = encodeSquare.mock.calls[0]!
        // фото 2000×1500 при zoom 2: виден квадрат 750 пикселей исходника
        expect(rect.size).toBeCloseTo(750)
        expect(onPick).toHaveBeenCalledWith(expect.any(Blob))
    })

    it('refuses a photo smaller than 512 px right away, with its size in the notice', async () => {
        loadImage.mockResolvedValue({ source: {}, width: 500, height: 900 })
        const { choose, onPick } = renderPicker()
        choose(photo())
        await waitFor(() =>
            expect(useNotifications.getState().items[0]?.title).toBe('Фото слишком маленькое'),
        )
        expect(useNotifications.getState().items[0]?.description).toMatch(/512×512.*500×900/)
        expect(onPick).not.toHaveBeenCalled()
        // окно закрывается (в разметке остаётся на время анимации)
        expect(screen.getByRole('dialog', { name: 'Фото' }).parentElement).toHaveAttribute(
            'data-state',
            'closed',
        )
    })

    it('limits the zoom so that the crop never gets smaller than 512 px', async () => {
        loadImage.mockResolvedValue({ source: {}, width: 1024, height: 768 })
        const { choose } = renderPicker()
        choose(photo())
        const slider = await screen.findByRole('slider', { name: 'Масштаб' })
        await waitFor(() => expect(slider).toBeEnabled())
        // короткая сторона 768: больше 768 / 512 = 1,5 приближать нельзя
        expect(Number(slider.getAttribute('max'))).toBeCloseTo(1.5)
    })

    it('closes with a notice when the photo cannot be decoded', async () => {
        loadImage.mockRejectedValue(new Error('broken'))
        const { choose, onPick } = renderPicker()
        choose(photo())
        await waitFor(() =>
            expect(useNotifications.getState().items[0]?.title).toBe('Не удалось открыть фото'),
        )
        expect(onPick).not.toHaveBeenCalled()
    })

    it('goes back without picking', async () => {
        const { choose, onPick } = renderPicker()
        choose(photo())
        fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
        expect(onPick).not.toHaveBeenCalled()
    })
})
