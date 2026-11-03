import { useEffect, type ReactNode } from 'react'
import { App as AntdApp, ConfigProvider, Watermark, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import { useAuth } from '../stores/auth'
import { useUi } from '../stores/ui'

const fontFamily = '"Avenir Next", "SF Pro Display", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const themeMode = useUi((state) => state.theme)
  const watermark = useUi((state) => state.watermark)
  const user = useAuth((state) => state.user)
  const dark = themeMode === 'dark'

  useEffect(() => {
    document.documentElement.dataset.theme = themeMode
  }, [themeMode])

  const body = (
    <AntdApp>
      <div className="app-watermark-layer">{children}</div>
    </AntdApp>
  )

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#ff5f33',
          colorInfo: '#2fb8d9',
          colorSuccess: '#2fb98a',
          colorWarning: '#f3a93c',
          colorError: '#e14b4b',
          colorText: dark ? '#eaf4fb' : '#0b1c2c',
          colorTextSecondary: dark ? '#a7bdcd' : '#5b7186',
          colorBgContainer: dark ? 'rgba(13, 33, 51, .48)' : 'rgba(255, 255, 255, .38)',
          colorBgElevated: dark ? 'rgba(15, 36, 55, .96)' : 'rgba(250, 253, 255, .94)',
          colorBorder: dark ? 'rgba(255,255,255,.16)' : 'rgba(255,255,255,.72)',
          colorSplit: dark ? 'rgba(255,255,255,.1)' : 'rgba(255,255,255,.5)',
          borderRadius: 12,
          borderRadiusLG: 20,
          controlHeight: 34,
          fontFamily,
          fontSize: 13,
          boxShadow: '0 10px 30px rgba(23, 43, 63, .1)',
          boxShadowSecondary: '0 24px 60px rgba(23, 43, 63, .16)',
        },
        components: {
          Button: {
            borderRadius: 999,
            borderRadiusSM: 999,
            primaryShadow: '0 10px 24px rgba(255, 95, 51, .3)',
            defaultBg: dark ? 'rgba(255,255,255,.08)' : 'rgba(255,255,255,.42)',
            defaultBorderColor: dark ? 'rgba(255,255,255,.16)' : 'rgba(255,255,255,.72)',
            defaultColor: dark ? '#eaf4fb' : '#0b1c2c',
          },
          Table: {
            headerBg: 'transparent',
            headerColor: dark ? '#a7bdcd' : '#5b7186',
            headerSplitColor: 'transparent',
            borderColor: dark ? 'rgba(255,255,255,.1)' : 'rgba(255,255,255,.5)',
            rowHoverBg: dark ? 'rgba(255,255,255,.05)' : 'rgba(255,255,255,.3)',
            colorBgContainer: 'transparent',
            cellPaddingBlock: 10,
          },
          Input: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: '#ff5f33',
            hoverBorderColor: 'rgba(255,95,51,.45)',
            activeShadow: '0 0 0 3px rgba(255,95,51,.12)',
          },
          InputNumber: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: '#ff5f33',
            hoverBorderColor: 'rgba(255,95,51,.45)',
            activeShadow: '0 0 0 3px rgba(255,95,51,.12)',
          },
          Select: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            optionSelectedBg: 'rgba(255,95,51,.12)',
            optionActiveBg: dark ? 'rgba(255,255,255,.08)' : 'rgba(255,255,255,.5)',
          },
          DatePicker: {
            colorBgContainer: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.38)',
            activeBorderColor: '#ff5f33',
            hoverBorderColor: 'rgba(255,95,51,.45)',
          },
          Modal: {
            contentBg: dark ? 'rgba(15,36,55,.97)' : 'rgba(250,253,255,.97)',
            headerBg: 'transparent',
            borderRadiusLG: 24,
          },
          Drawer: {
            colorBgElevated: dark ? 'rgba(15,36,55,.96)' : 'rgba(248,252,255,.94)',
          },
          Dropdown: {
            colorBgElevated: dark ? 'rgba(15,36,55,.96)' : 'rgba(250,253,255,.94)',
          },
          Tree: {
            nodeSelectedBg: 'rgba(255,95,51,.12)',
            nodeHoverBg: dark ? 'rgba(255,255,255,.06)' : 'rgba(255,255,255,.4)',
            colorBgContainer: 'transparent',
          },
          Pagination: {
            itemBg: 'transparent',
            itemActiveBg: 'rgba(255,95,51,.14)',
          },
          Tabs: {
            inkBarColor: '#ff5f33',
          },
          Switch: { colorPrimary: '#ff5f33' },
          Tag: {
            defaultBg: dark ? 'rgba(255,255,255,.08)' : 'rgba(255,255,255,.42)',
            defaultColor: dark ? '#c6d8e5' : '#5b7186',
          },
          Popover: { colorBgElevated: dark ? 'rgba(15,36,55,.96)' : 'rgba(250,253,255,.96)' },
          Message: { contentBg: dark ? 'rgba(15,36,55,.96)' : 'rgba(250,253,255,.96)' },
        },
      }}
    >
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
