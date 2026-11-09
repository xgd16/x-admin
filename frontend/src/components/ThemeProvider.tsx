/**
 * antd 主题、中文语言包、水印与主题色板注入。
 */
import { useEffect, type ReactNode } from 'react'
import { ConfigProvider, Watermark, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import { useReducedMotion } from 'motion/react'
import { paletteOf, tint } from '../config/themes'
import { useAuth } from '../stores/auth'
import { useUi } from '../stores/ui'
import { mountGlassRefraction, supportsBackdropRefraction } from '../utils/liquidGlass'

// 与全局样式保持一致的字体栈。
const fontFamily = '"Avenir Next", "SF Pro Display", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const themeMode = useUi((state) => state.theme)
  const palette = useUi((state) => state.palette)
  const watermark = useUi((state) => state.watermark)
  const liquidGlass = useUi((state) => state.liquidGlass)
  const user = useAuth((state) => state.user)
  const dark = themeMode === 'dark'
  const reduceMotion = useReducedMotion()
  const accent = paletteOf(palette).accent

  // 把主题与色板写到 html 属性上，CSS 变量、暗色选择器与 data-palette 都依赖它们。
  useEffect(() => {
    document.documentElement.dataset.theme = themeMode
    document.documentElement.dataset.palette = palette
    document.documentElement.dataset.liquidGlass = liquidGlass ? 'on' : 'off'
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', themeMode === 'dark' ? '#070c15' : '#edf3f9')
  }, [themeMode, palette, liquidGlass])

  // 液态玻璃开启时给所有 .glass 面板挂上边缘折射；Chromium 之外自动退回普通玻璃。
  useEffect(() => {
    if (!liquidGlass || !supportsBackdropRefraction()) return
    return mountGlassRefraction(document.body)
  }, [liquidGlass])

  // Toast 的位置由 Router 内的 ToastScope 决定：后台避让吸顶导航，登录页没有框架导航。
  const body = <div className="app-watermark-layer">{children}</div>

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        // 通过 antd 算法 + token 覆盖，让组件默认样式与玻璃拟态主题一致。
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: accent,
          colorInfo: '#2fb8d9',
          colorSuccess: '#2fb98a',
          colorWarning: '#f3a93c',
          colorError: '#e14b4b',
          colorText: dark ? '#f3f7fc' : '#0b1c2c',
          colorTextSecondary: dark ? '#a0b1c3' : '#5b7186',
          colorBgContainer: dark ? 'rgba(19, 27, 40, .5)' : 'rgba(255, 255, 255, .38)',
          colorBgElevated: dark ? 'rgba(18, 26, 39, .97)' : 'rgba(250, 253, 255, .94)',
          colorBorder: dark ? 'rgba(156, 182, 210, .16)' : 'rgba(255,255,255,.72)',
          colorSplit: dark ? 'rgba(156, 182, 210, .1)' : 'rgba(255,255,255,.5)',
          borderRadius: 12,
          borderRadiusLG: 20,
          controlHeight: 34,
          fontFamily,
          fontSize: 13,
          motion: !reduceMotion,
          boxShadow: dark ? '0 10px 30px rgba(0, 0, 0, .35)' : '0 10px 30px rgba(23, 43, 63, .1)',
          boxShadowSecondary: dark ? '0 24px 60px rgba(0, 0, 0, .45)' : '0 24px 60px rgba(23, 43, 63, .16)',
        },
        components: {
          Button: {
            borderRadius: 999,
            borderRadiusSM: 999,
            primaryShadow: `0 10px 24px ${tint(accent, 0.3)}`,
            defaultBg: dark ? 'rgba(255,255,255,.07)' : 'rgba(255,255,255,.42)',
            defaultBorderColor: dark ? 'rgba(156, 182, 210, .16)' : 'rgba(255,255,255,.72)',
            defaultColor: dark ? '#f3f7fc' : '#0b1c2c',
          },
          Table: {
            headerBg: 'transparent',
            headerColor: dark ? '#9fb0c3' : '#5b7186',
            headerSplitColor: 'transparent',
            borderColor: dark ? 'rgba(156, 182, 210, .1)' : 'rgba(255,255,255,.5)',
            rowHoverBg: dark ? 'rgba(255,255,255,.05)' : 'rgba(255,255,255,.3)',
            colorBgContainer: 'transparent',
            cellPaddingBlock: 10,
          },
          Input: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: accent,
            hoverBorderColor: tint(accent, 0.45),
            activeShadow: `0 0 0 3px ${tint(accent, 0.12)}`,
          },
          InputNumber: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: accent,
            hoverBorderColor: tint(accent, 0.45),
            activeShadow: `0 0 0 3px ${tint(accent, 0.12)}`,
          },
          Select: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            optionSelectedBg: tint(accent, 0.12),
            optionActiveBg: dark ? 'rgba(255,255,255,.08)' : 'rgba(255,255,255,.5)',
          },
          DatePicker: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: accent,
            hoverBorderColor: tint(accent, 0.45),
          },
          Modal: {
            contentBg: dark ? 'rgba(18,26,39,.97)' : 'rgba(250,253,255,.97)',
            headerBg: 'transparent',
            borderRadiusLG: 24,
          },
          Drawer: {
            colorBgElevated: dark ? 'rgba(18,26,39,.97)' : 'rgba(248,252,255,.94)',
          },
          Dropdown: {
            colorBgElevated: dark ? 'rgba(18,26,39,.97)' : 'rgba(250,253,255,.94)',
          },
          Tree: {
            nodeSelectedBg: tint(accent, 0.12),
            nodeHoverBg: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.4)',
            colorBgContainer: 'transparent',
          },
          Pagination: {
            itemBg: 'transparent',
            itemActiveBg: tint(accent, 0.14),
          },
          Tabs: {
            inkBarColor: accent,
          },
          Switch: { colorPrimary: accent },
          Tag: {
            defaultBg: dark ? 'rgba(255,255,255,.07)' : 'rgba(255,255,255,.42)',
            defaultColor: dark ? '#c3d1df' : '#5b7186',
          },
          Popover: { colorBgElevated: dark ? 'rgba(18,26,39,.97)' : 'rgba(250,253,255,.96)' },
          Message: { contentBg: dark ? 'rgba(18,26,39,.97)' : 'rgba(250,253,255,.96)' },
          Skeleton: {
            gradientFromColor: dark ? 'rgba(255,255,255,.055)' : 'rgba(11,28,44,.045)',
            gradientToColor: dark ? 'rgba(255,255,255,.12)' : 'rgba(11,28,44,.095)',
            blockRadius: 9,
          },
        },
      }}
    >
      {/* 水印按需包裹整个应用，文案包含当前昵称和日期。 */}
      {watermark ? (
        <Watermark
          content={[`x-admin · ${user?.nickname ?? 'guest'}`, dayjs().format('YYYY-MM-DD')]}
          font={{ color: dark ? 'rgba(255,255,255,.06)' : 'rgba(11,28,44,.06)', fontSize: 14 }}
          gap={[120, 120]}
          rotate={-18}
        >
          {body}
        </Watermark>
      ) : (
        body
      )}
    </ConfigProvider>
  )
}
