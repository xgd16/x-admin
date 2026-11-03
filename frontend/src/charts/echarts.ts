import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

// Only the pieces the workbench actually draws, so the async chart chunk stays
// small instead of pulling the full ECharts build.
echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

export { echarts }
