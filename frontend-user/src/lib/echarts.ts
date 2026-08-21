import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  MarkLineComponent,
  MarkPointComponent,
} from 'echarts/components'

use([CanvasRenderer, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, MarkLineComponent, MarkPointComponent])

export const chartBase = {
  textStyle: {
    color: '#C9BFAF',
    fontFamily: '"Source Sans 3", sans-serif',
  },
  grid: { left: 36, right: 16, top: 28, bottom: 32, containLabel: true },
  tooltip: {
    trigger: 'axis' as const,
    backgroundColor: '#1B2030',
    borderColor: 'rgba(243,235,224,0.08)',
    textStyle: { color: '#F3EBE0' },
  },
}
