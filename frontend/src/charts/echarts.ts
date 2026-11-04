/**
 * ECharts 按需注册，减小图表异步包体积。
 */
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

// 只注册工作台实际用到的图表与组件，避免把完整 ECharts 打进异步包。
echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

export { echarts }
