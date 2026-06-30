# 键盘快捷键

本文档描述 tokui 支持的所有键盘快捷键，以及它们在不同 UI 模式下的行为。

## UI 模式

| 模式 | 说明 |
|------|------|
| `PENDING` | 初始加载状态，不处理任何输入。 |
| `READY` | 主浏览模式。 |
| `INPUT` | 快速名称过滤模式（按 `/` 进入）。 |
| `PREVIEW` | 文件内容预览模式。 |
| `SEARCH` | 全局模糊搜索模式（按 `Ctrl+P` 进入）。 |
| `SELECT_LANG` | 语言多选弹窗（按 `Ctrl+L` 进入）。 |

除上述模式外，还有几个视图状态标志：

- `treeMode` —— 树形可展开目录视图。
- `treemapMode` —— 矩形树图视图。
- `showCart` —— 语言占比饼图浮层。
- `fullHelp` —— 展开的帮助面板。
- `treemapColorByLang` —— 树图配色切换。
- `treemapSizeKey` —— 树图块大小指标（Total / Complexity / Bytes）。

---

## 顶层全局行为（`ViewModel`）

这些按键会在当前模式处理之前被优先分发。

| 按键 | 行为 |
|-----|------|
| `Ctrl+C` | 无条件退出应用。 |
| `q` | 仅在 `READY` / `TREE` / `TREEMAP` 模式下退出；在 `INPUT` / `SEARCH` / `PREVIEW` 模式下忽略。 |
| `Enter` | 确认 / 打开 / 展开当前选中项（具体行为取决于当前模式，见下表）。 |
| `Backspace` | 返回上级目录；在 `INPUT` 和 `SEARCH` 模式下不处理。 |

### `Enter` 详细行为

| 当前模式 | 行为 |
|----------|------|
| `INPUT` | 关闭过滤框，然后对当前选中项执行进入/展开/预览。 |
| `SEARCH` | 跳转到选中的搜索结果并关闭搜索弹窗。 |
| `READY` / `TREE` / `TREEMAP` | 选中 `..` → 返回上级；Tree 模式 → 展开/折叠目录；Treemap → 钻取；否则 → 进入目录或预览文件。 |

---

## `READY` 模式（主浏览）

| 按键 | 功能 |
|------|------|
| `↑` / `↓` / `k` / `j` | 上下移动选中行。 |
| `home` / `g` | 跳到列表顶部。 |
| `end` / `G` | 跳到列表底部。 |
| `Enter` | 进入目录 / 预览文件 / 展开或折叠。 |
| `Backspace` | 返回上级目录。 |
| `e` | 使用 `$EDITOR`（默认 `vim`）打开当前文件。 |
| `/` | 进入 `INPUT` 名称过滤模式。 |
| `Ctrl+P` | 打开 `SEARCH` 全局模糊搜索。 |
| `Tab` | 循环切换语言过滤（`All` → 语言 1 → 语言 2 → … → `All`）。 |
| `Ctrl+L` | 打开 `SELECT_LANG` 多语言选择弹窗。 |
| `Ctrl+W` | 显示或隐藏语言占比饼图。 |
| `t` | 切换 Tree 模式。 |
| `m` | 切换 Treemap 模式。 |
| `c` | 切换 Treemap 颜色模式（按目录 / 按语言）。 |
| `M` | 循环 Treemap 块大小指标（Total → Complexity → Bytes，需 scc Provider）。 |
| `s` | 循环排序列。 |
| `S` | 切换当前排序列的升序/降序。 |
| `?` | 显示/隐藏完整帮助面板。 |
| `q` / `Ctrl+C` | 退出应用。 |

### 排序顺序

按 `s` 会在以下列之间循环：

`Name` → `Languages` → `Code` → `Comments` → `Blanks` → `Total` → `Percent` → `Complexity`

按 `S` 切换方向。文本列默认升序，数值列默认降序。

`% of Parent` 列的分母会随当前排序列变化：默认按代码行数总计，按 `Complexity` 排序时按复杂度总计。

---

## `INPUT` 模式（名称过滤）

在 `READY` 模式下按 `/` 进入。

| 按键 | 功能 |
|------|------|
| 可打印字符 / 数字 | 输入过滤文本，实时缩小当前目录列表。 |
| `Esc` | 清除过滤并返回 `READY`。 |
| `Enter` | 返回 `READY` 并对当前选中项执行进入/展开/预览。 |
| `Backspace` | 删除上一个过滤字符。 |
| `q` | 作为过滤字符输入，**不退出**。 |
| `Ctrl+C` | 退出应用。 |



---

## `SEARCH` 模式（全局模糊搜索）

在 `READY` 模式下按 `Ctrl+P` 进入。

| 按键 | 功能 |
|------|------|
| 可打印字符 / 数字 | 输入搜索关键字，全项目模糊匹配文件/目录。 |
| `↑` / `k` | 上一个搜索结果。 |
| `↓` / `j` | 下一个搜索结果。 |
| `pgup` | 向上翻 10 条结果。 |
| `pgdown` | 向下翻 10 条结果。 |
| `home` / `g` | 跳到第一个结果。 |
| `end` / `G` | 跳到最后一个结果。 |
| `Enter` | 跳转到选中结果并关闭搜索弹窗。 |
| `Esc` | 关闭搜索并返回 `READY`。 |
| `q` | 作为搜索字符输入，**不退出**。 |
| `Ctrl+C` | 退出应用。 |

---

## `SELECT_LANG` 模式（语言选择弹窗）

在 `READY` 模式下按 `Ctrl+L` 进入。

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 在语言列表中向上移动。 |
| `↓` / `j` | 在语言列表中向下移动。 |
| `Space` | 选中/取消选中当前高亮语言。 |
| `Enter` | 确认选择，应用多语言过滤，返回 `READY`。 |
| `Esc` / `Ctrl+L` | 取消选择，关闭弹窗，返回 `READY`。 |
| `q` | 关闭弹窗，返回 `READY`。 |
| `Ctrl+C` | 退出应用。 |

---

## `PREVIEW` 模式（文件预览）

在 `READY` 模式下打开文件进入。

| 按键 | 功能 |
|------|------|
| `q` / `Esc` | 关闭预览，返回 `READY`。 |
| `↑` / `↓` / `j` / `k` | 向上/向下滚动一行。 |
| `pgup` / `pgdown` | 向上/向下滚动一页。 |
| `home` / `end` | 跳到文件顶部/底部。 |

> 预览底部提示显示 `q/Esc` 关闭，方向键/`PgUp`/`PgDn`/`Home`/`End` 导航。

---

## `TREEMAP` 视图专用按键

当 `treemapMode` 激活时，除正常 `READY` 行为外，还会处理以下按键：

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 在顶层块之间向上选择。 |
| `↓` / `j` | 在顶层块之间向下选择。 |
| `t` | 从 Treemap 切换回 Tree 模式。 |
| `m` | 关闭 Treemap 视图。 |
| `c` | 切换 Treemap 颜色模式。 |
| `M` | 循环 Treemap 块大小指标。 |

`Enter` 和 `Backspace` 仍由顶层 `ViewModel` 处理，分别用于钻取和返回上级。

---

## 快速参考

```text
READY / TREE / TREEMAP
├── 移动: ↑/↓/k/j, home/g, end/G
├── 进入: Enter
├── 返回: Backspace
├── 视图: t (tree), m (treemap), c (treemap 配色), M (treemap 大小指标)
├── 过滤: / (快速过滤), Tab (循环单语言), Ctrl+L (多选语言)
├── 搜索: Ctrl+P
├── 图表: Ctrl+W
├── 排序: s (换列), S (换方向)
├── 编辑: e
├── 帮助: ?
└── 退出: q, Ctrl+C

INPUT (/)
├── 输入过滤文本
├── Esc: 清除并返回
├── Enter: 确认并执行当前选中项
└── Ctrl+C: 退出

SEARCH (Ctrl+P)
├── 输入搜索关键字
├── ↑/↓/k/j/pgup/pgdown/home/end/g/G: 结果导航
├── Enter: 跳转
├── Esc: 关闭
└── Ctrl+C: 退出

SELECT_LANG (Ctrl+L)
├── ↑/↓/k/j: 移动
├── Space: 选中/取消
├── Enter: 确认
├── Esc/Ctrl+L/q: 取消/关闭
└── Ctrl+C: 退出

PREVIEW
├── q/Esc: 关闭
├── ↑/↓/j/k/pgup/pgdown: 滚动
└── home/end: 跳到文件首尾
```

---

## 已知限制

1. **终端快捷键冲突风险。** `Ctrl+L`（清屏）、`Ctrl+W`（删除前一个词）、`Ctrl+P`（历史搜索）在某些终端或 Shell 中可能被拦截。如果这些键不生效，用户可能需要调整终端配置。
