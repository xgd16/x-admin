/**
 * 液态玻璃：位移图数学与浏览器能力判断。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildDisplacementMap, supportsBackdropRefraction } from './liquidGlass'

interface CapturedMap {
  data: Uint8ClampedArray
}

/** 用假的 2D context 截获生成的 ImageData，避免依赖真实 canvas。 */
function captureMap(width: number, height: number, radius: number, bevel: number): CapturedMap {
  let captured: CapturedMap | null = null
  const ctx = {
    createImageData: (w: number, h: number) => ({ data: new Uint8ClampedArray(w * h * 4) }),
    putImageData: (image: { data: Uint8ClampedArray }) => {
      captured = { data: image.data }
    },
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(ctx as unknown as CanvasRenderingContext2D)
  vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue('data:image/png;base64,test')

  expect(buildDisplacementMap(width, height, radius, bevel)).toBe('data:image/png;base64,test')
  if (!captured) throw new Error('displacement map was not generated')
  return captured
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('buildDisplacementMap', () => {
  it('keeps the center neutral and bends the edge inward', () => {
    const map = captureMap(200, 200, 24, 20)
    // 长边固定为 360，正方形地图为 360x360。
    const size = 360
    const at = (x: number, y: number) => {
      const index = (y * size + x) * 4
      return { r: map.data[index], g: map.data[index + 1] }
    }

    const center = at(180, 180)
    expect(center.r).toBe(128)
    expect(center.g).toBe(128)

    // 左缘中点的法线指向 -x，位移指向面板内部（+x），R 通道应明显大于 128。
    const leftEdge = at(1, 180)
    expect(leftEdge.r).toBeGreaterThan(200)
    expect(Math.abs(leftEdge.g - 128)).toBeLessThan(4)

    // 圆角外侧保持中性，避免把面板外的内容卷进来。
    const corner = at(1, 1)
    expect(corner.r).toBe(128)
    expect(corner.g).toBe(128)
  })

  it('encodes vertical displacement on the top edge', () => {
    const map = captureMap(200, 200, 24, 20)
    const size = 360
    const index = (1 * size + 180) * 4 // 上缘中点，位移指向 +y
    expect(map.data[index + 1]).toBeGreaterThan(200)
  })
})

describe('supportsBackdropRefraction', () => {
  it('accepts Chromium when backdrop url filters are supported', () => {
    vi.stubGlobal('CSS', { supports: () => true })
    vi.stubGlobal('navigator', {
      userAgentData: { brands: [{ brand: 'Chromium' }] },
      userAgent: 'Chrome/141',
    })
    expect(supportsBackdropRefraction()).toBe(true)
  })

  it('rejects engines that accept the syntax but do not render it', () => {
    vi.stubGlobal('CSS', { supports: () => true })
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (Macintosh) AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
    })
    expect(supportsBackdropRefraction()).toBe(false)

    vi.stubGlobal('navigator', { userAgent: 'Mozilla/5.0 (X11; Linux) Gecko/20100101 Firefox/141.0' })
    expect(supportsBackdropRefraction()).toBe(false)
  })

  it('requires CSS.supports to exist', () => {
    vi.stubGlobal('navigator', { userAgentData: { brands: [{ brand: 'Chromium' }] }, userAgent: 'Chrome/141' })
    vi.stubGlobal('CSS', {})
    expect(supportsBackdropRefraction()).toBe(false)
  })
})
