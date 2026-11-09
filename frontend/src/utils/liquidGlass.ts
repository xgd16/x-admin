/**
 * 液态玻璃（Liquid Glass）能力探测与边缘折射。
 *
 * 思路参考 Apple WWDC25 的 Liquid Glass 与社区在 Web 上的还原方式：
 * - 面板边缘的「折射」用一张圆角矩形高度场生成的位移图（feDisplacementMap）实现，
 *   通过 backdrop-filter: url(#filter) 作用在面板背后的真实内容上；
 * - 浏览器不支持 SVG 作为 backdrop-filter 时自动退回普通玻璃拟态，不影响可用性。
 */

const SVG_NS = 'http://www.w3.org/2000/svg'
const FILTER_PREFIX = 'lg-refract-'
const MAP_LONG_SIDE = 360
/** 位移图按尺寸分桶缓存，避免拖拽窗口时不断生成新滤镜。 */
const SIZE_BUCKET = 8
/** 滚动停止后延迟恢复折射，避免惯性滚动中反复切换。 */
const SCROLL_RESTORE_MS = 160

/** 是否支持把 SVG 滤镜当作 backdrop-filter（目前只有 Chromium 系会真正渲染）。 */
export function supportsBackdropRefraction(): boolean {
  if (typeof navigator === 'undefined' || typeof CSS === 'undefined' || !CSS.supports) return false
  // Firefox/Safari 会接受 url() 语法但不渲染，必须显式限定引擎，否则会失去磨砂兜底。
  const brands = (navigator as Navigator & { userAgentData?: { brands?: Array<{ brand: string }> } })
    .userAgentData?.brands
  const chromium = brands
    ? brands.some((item) => /Chromium|Google Chrome|Microsoft Edge/i.test(item.brand))
    : /Chrome|Chromium|Edg|OPR/i.test(navigator.userAgent) && !/CriOS|FxiOS/i.test(navigator.userAgent)
  if (!chromium) return false
  try {
    return CSS.supports('backdrop-filter', 'url(#lg-probe)') || CSS.supports('-webkit-backdrop-filter', 'url(#lg-probe)')
  } catch {
    return false
  }
}

/* ---------------- 位移图 ---------------- */

/** 圆角矩形的有符号距离场：内部为负，外部为正。 */
function roundedRectSdf(px: number, py: number, width: number, height: number, radius: number) {
  const qx = Math.abs(px - width / 2) - (width / 2 - radius)
  const qy = Math.abs(py - height / 2) - (height / 2 - radius)
  const outside = Math.hypot(Math.max(qx, 0), Math.max(qy, 0))
  return outside + Math.min(Math.max(qx, qy), 0) - radius
}

/**
 * 生成圆角矩形边缘的折射位移图。
 *
 * 每个像素的 R/G 通道编码位移方向（128 为 0），位移指向面板内部：
 * 边缘处最强、向内衰减到 0，模拟凸起玻璃斜面把背景压缩进边缘的效果。
 */
export function buildDisplacementMap(width: number, height: number, radius: number, bevel: number): string {
  const ratio = width >= height ? MAP_LONG_SIDE / width : MAP_LONG_SIDE / height
  const mapWidth = Math.max(8, Math.round(width * ratio))
  const mapHeight = Math.max(8, Math.round(height * ratio))
  const canvas = document.createElement('canvas')
  canvas.width = mapWidth
  canvas.height = mapHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) return ''

  const image = ctx.createImageData(mapWidth, mapHeight)
  const data = image.data
  const stepX = width / mapWidth
  const stepY = height / mapHeight
  const epsilon = Math.max(0.75, Math.min(width, height) / 400)

  for (let y = 0; y < mapHeight; y += 1) {
    for (let x = 0; x < mapWidth; x += 1) {
      const px = (x + 0.5) * stepX
      const py = (y + 0.5) * stepY
      const distance = roundedRectSdf(px, py, width, height, radius)
      const index = (y * mapWidth + x) * 4
      // 圆角外侧保持中性色，避免把面板外的内容卷进来。
      if (distance >= 0) {
        data[index] = 128
        data[index + 1] = 128
        data[index + 2] = 128
        data[index + 3] = 255
        continue
      }

      const edge = -distance
      const t = Math.min(1, edge / bevel)
      const magnitude = Math.pow(1 - t, 0.85)

      // 距离场梯度指向外侧，取反即为指向面板内部的方向。
      const gx = roundedRectSdf(px + epsilon, py, width, height, radius) -
        roundedRectSdf(px - epsilon, py, width, height, radius)
      const gy = roundedRectSdf(px, py + epsilon, width, height, radius) -
        roundedRectSdf(px, py - epsilon, width, height, radius)
      const length = Math.hypot(gx, gy) || 1
      const nx = (-gx / length) * magnitude
      const ny = (-gy / length) * magnitude

      data[index] = Math.round(128 + nx * 127)
      data[index + 1] = Math.round(128 + ny * 127)
      data[index + 2] = 128
      data[index + 3] = 255
    }
  }

  ctx.putImageData(image, 0, 0)
  return canvas.toDataURL('image/png')
}

/* ---------------- 滤镜挂载 ---------------- */

interface RefractionEntry {
  key: string
  id: string
}

function readRadius(element: HTMLElement, width: number, height: number): number {
  const raw = getComputedStyle(element).borderTopLeftRadius || '0px'
  const value = Number.parseFloat(raw)
  if (!Number.isFinite(value)) return 0
  return raw.trim().endsWith('%') ? (Math.min(width, height) * value) / 100 : value
}

function roundBucket(value: number): number {
  return Math.max(1, Math.round(value / SIZE_BUCKET) * SIZE_BUCKET)
}

/**
 * 给容器内所有 `.glass` 面板挂上边缘折射滤镜。
 *
 * 只在液态玻璃开启且浏览器支持时调用；返回清理函数。
 */
export function mountGlassRefraction(container: HTMLElement): () => void {
  const defs = document.createElementNS(SVG_NS, 'svg')
  defs.setAttribute('aria-hidden', 'true')
  defs.setAttribute('width', '0')
  defs.setAttribute('height', '0')
  defs.style.position = 'absolute'
  defs.style.width = '0'
  defs.style.height = '0'
  defs.style.overflow = 'hidden'
  defs.style.pointerEvents = 'none'
  document.body.appendChild(defs)

  const filters = new Map<string, string>() // key -> filter id
  const entries = new Map<HTMLElement, RefractionEntry>()
  let filterSeq = 0
  let disposed = false

  const ensureFilter = (width: number, height: number, radius: number): { id: string; key: string } | null => {
    const w = roundBucket(width)
    const h = roundBucket(height)
    const r = Math.max(0, Math.min(Math.round(radius), Math.min(w, h) / 2))
    const bevel = Math.max(12, Math.min(40, Math.min(w, h) * 0.13))
    const key = `${w}x${h}:${r}`
    const existing = filters.get(key)
    if (existing) return { id: existing, key }

    const href = buildDisplacementMap(w, h, r, bevel)
    if (!href) return null
    filterSeq += 1
    const id = `${FILTER_PREFIX}${filterSeq}`
    const filter = document.createElementNS(SVG_NS, 'filter')
    filter.setAttribute('id', id)
    filter.setAttribute('x', '0')
    filter.setAttribute('y', '0')
    filter.setAttribute('width', '100%')
    filter.setAttribute('height', '100%')
    filter.setAttribute('filterUnits', 'objectBoundingBox')
    filter.setAttribute('primitiveUnits', 'userSpaceOnUse')
    filter.setAttribute('color-interpolation-filters', 'sRGB')

    const image = document.createElementNS(SVG_NS, 'feImage')
    image.setAttribute('href', href)
    image.setAttribute('x', '0')
    image.setAttribute('y', '0')
    // 必须用具体像素：defs SVG 尺寸为 0 时，百分比会在 userSpaceOnUse 下解析成 0，
    // feImage 拿不到有效图像，折射就会静默失效。
    image.setAttribute('width', String(w))
    image.setAttribute('height', String(h))
    image.setAttribute('preserveAspectRatio', 'none')
    image.setAttribute('result', 'map')

    const blur = document.createElementNS(SVG_NS, 'feGaussianBlur')
    blur.setAttribute('in', 'SourceGraphic')
    blur.setAttribute('stdDeviation', '6')
    blur.setAttribute('result', 'frosted')

    const displace = document.createElementNS(SVG_NS, 'feDisplacementMap')
    displace.setAttribute('in', 'frosted')
    displace.setAttribute('in2', 'map')
    displace.setAttribute('scale', String(Math.max(16, Math.min(48, bevel * 1.1))))
    displace.setAttribute('xChannelSelector', 'R')
    displace.setAttribute('yChannelSelector', 'G')
    displace.setAttribute('result', 'refracted')

    const saturate = document.createElementNS(SVG_NS, 'feColorMatrix')
    saturate.setAttribute('in', 'refracted')
    saturate.setAttribute('type', 'saturate')
    saturate.setAttribute('values', '1.6')

    filter.append(image, blur, displace, saturate)
    defs.appendChild(filter)
    filters.set(key, id)
    return { id, key }
  }

  const apply = (element: HTMLElement) => {
    if (!element.classList.contains('glass')) return
    const rect = element.getBoundingClientRect()
    // 隐藏或尚未布局的元素跳过，等 ResizeObserver / MutationObserver 再次触发。
    if (rect.width < 24 || rect.height < 24) return
    const radius = readRadius(element, rect.width, rect.height)
    const next = ensureFilter(rect.width, rect.height, radius)
    if (!next) return
    const previous = entries.get(element)
    if (previous?.key === next.key) return
    element.style.setProperty('--lg-refract', `url(#${next.id})`)
    entries.set(element, { key: next.key, id: next.id })
  }

  let timer = 0
  const scan = () => {
    timer = 0
    if (disposed) return
    // 页面卸载后清理条目，避免 Map 持有已移除的 DOM。
    entries.forEach((_entry, element) => {
      if (!element.isConnected) {
        element.style.removeProperty('--lg-refract')
        entries.delete(element)
      }
    })
    container.querySelectorAll<HTMLElement>('.glass').forEach(apply)

    // 回收当前没有任何面板使用的尺寸滤镜。
    const used = new Set(Array.from(entries.values(), (entry) => entry.key))
    filters.forEach((id, key) => {
      if (used.has(key)) return
      document.getElementById(id)?.remove()
      filters.delete(key)
    })

    // 首次成功挂载后让 CSS 切到折射路径；全失败则保持普通玻璃。
    if (entries.size > 0) document.documentElement.dataset.lgRefract = 'on'
  }
  const scheduleScan = () => {
    if (timer || disposed) return
    timer = window.setTimeout(scan, 180)
  }

  // 内容滚动时，每个 backdrop-filter 都要按新位置重新采样整块背景。
  // SVG 位移滤镜（高斯模糊 + 位移映射）比普通 blur 贵得多，很容易把 GPU 打满。
  // 因此滚动期间先关掉内容区的实时模糊，停下后再恢复折射。
  const root = document.documentElement
  let scrollTimer = 0
  const startScroll = () => {
    if (root.dataset.lgScrolling !== 'on') root.dataset.lgScrolling = 'on'
    window.clearTimeout(scrollTimer)
    scrollTimer = window.setTimeout(() => {
      scrollTimer = 0
      delete root.dataset.lgScrolling
    }, SCROLL_RESTORE_MS)
  }
  // 只响应主内容区的滚动：表格、下拉框等内部滚动不会带动整个页面，
  // 它们自己的实时模糊（如表格固定列）需要保留，不能跟着降级。
  const onScroll = (event: Event) => {
    const target = event.target
    if (!(target instanceof HTMLElement) || !target.classList.contains('canvas-body')) return
    startScroll()
  }
  document.addEventListener('scroll', onScroll, { capture: true, passive: true })

  const observer = new MutationObserver(scheduleScan)
  observer.observe(container, { childList: true, subtree: true })

  const resize = new ResizeObserver(scheduleScan)
  const resizeTargets = new WeakSet<HTMLElement>()
  const track = () => {
    container.querySelectorAll<HTMLElement>('.glass').forEach((element) => {
      if (!resizeTargets.has(element)) {
        resizeTargets.add(element)
        resize.observe(element)
      }
    })
  }

  const onResize = () => {
    track()
    scheduleScan()
  }

  scan()
  track()
  window.addEventListener('resize', onResize)

  return () => {
    disposed = true
    observer.disconnect()
    resize.disconnect()
    window.clearTimeout(timer)
    window.clearTimeout(scrollTimer)
    window.removeEventListener('resize', onResize)
    document.removeEventListener('scroll', onScroll, { capture: true })
    delete root.dataset.lgScrolling
    entries.forEach((_entry, element) => {
      element.style.removeProperty('--lg-refract')
    })
    defs.remove()
    delete document.documentElement.dataset.lgRefract
  }
}
