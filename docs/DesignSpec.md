# GoJira Design Spec

**方向：Workshop Dusk（工坊黄昏）** — 暖纸卡片铺在墨色工作台上，铜橙作行动色。刻意避开 Inter / 紫渐变 / 通用 SaaS 白底。

## 1. 调色

| Token | 值 | 用途 |
|---|---|---|
| `--ink` | `#12151C` | 主背景、侧栏 |
| `--ink-2` | `#1B2030` | 升高表面 |
| `--ink-3` | `#252C3D` | 列槽、输入底 |
| `--paper` | `#F3EBE0` | 卡片、主文字反色 |
| `--paper-dim` | `#C9BFAF` | 次级文字 |
| `--copper` | `#D46A2C` | 主按钮、焦点环、今日线 |
| `--copper-deep` | `#A44B18` | 按钮按下 |
| `--teal` | `#2A9D8F` | 开发中 / 进行 |
| `--gold` | `#E9C46A` | 已测试 / 警告 |
| `--olive` | `#7C9A6C` | 已完成 |
| `--rose` | `#C45C5C` | 逾期、Blocker、错误 |
| `--line` | `rgba(243,235,224,0.08)` | 分割线 |

优先级色标：Blocker/Highest=`--rose`，High=`--copper`，Medium=`--gold`，Low=`--teal`，Lowest=`--paper-dim`。

## 2. 字体

- Display：`Fraunces`（卡片编号、页面大标题）
- Body：`Source Sans 3`（UI 正文）
- Mono：`IBM Plex Mono`（Issue Key `GJ-1024`）
- 禁止 Inter / Roboto / Arial / Space Grotesk。

CDN：Google Fonts `Fraunces:600,700` + `Source+Sans+3:400,500,600` + `IBM+Plex+Mono:500`。

## 3. 布局

- 左栏 232px：项目切换、看板 / 待办 / 甘特 / 统计 / 触发器日志。
- 顶栏 56px：项目名、Sprint 徽章、当前用户、角色色点。
- 主区全宽看板：四列等分，列头显示中文名 + 次级 hint（「已测试」下写「测试验证中」）+ 卡片数 + 故事点小计。
- 卡片：圆角 10px，暖纸底，左侧 3px 类型色条（Story 铜 / Task 青 / Bug 玫）。
- 抽屉：右侧 480px 滑入，编辑 Issue。

## 4. 组件

- 按钮：铜底纸字（主）、墨底描边（次）、玫瑰描边（危险）。hover 上浮 1px + 阴影。
- 输入：ink-3 底、focus copper 环 2px。
- Toast：右下，成功橄榄 / 失败玫瑰 / 警告金。
- 空态：虚线列槽 + 一句短文案，无插画。
- 拖拽：幽灵卡 0.55 透明，目标列 copper 虚线框。
- 甘特：SVG，行高 36，今日铜竖线，逾期条玫瑰，FS 依赖用贝塞尔曲线。
- 图表：ECharts，背景透明，轴文字 paper-dim，理想线虚线 paper-dim，实际线铜实线。

## 5. 动效

- 页面进入：主区 180ms fade+4px 上移。
- 列间落位：160ms ease-out。
- 抽屉：220ms transform。
- 禁止大面积循环动画。

## 6. 成本可见性

本项目无按量外部 API，控件不展示费用。邮件走自托管 SMTP。

## 7. 响应式

- ≥1280：完整三区。
- 768–1279：侧栏可折叠为图标轨。
- <768：列横向滚动，抽屉改全屏。
