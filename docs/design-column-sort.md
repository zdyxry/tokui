# Tokui 按列排序功能设计方案

## 1. 目标

在主视图（目录列表 / 树形视图）的表格中，允许用户按任意可排序列对当前目录的子项进行升序 / 降序排序，并在列标题上显示排序指示符（`▲` / `▼`）。

## 2. 当前状态

- `render/column.go` 已预留：
  - `SortKey string`
  - `SortState{Key SortKey; Desc bool}`
  - `Column.FmtName()` 已支持在标题后追加 ` ▲` / ` ▼`
- `DirModel.columns` 定义了 9 列，但 **没有设置 `SortKey`**。
- `DirModel.updateTableData()` 中硬编码调用 `dm.nav.Entry().SortChild()`，仅按 `TotalStats.Total()` 降序排列。
- `structure.Entry.SortChild()` 只支持“按 Total 降序”一种排序。
- 键绑定中没有排序相关快捷键。

因此本方案主要任务是：**把预留的排序模型真正接入数据层与交互层**。

## 3. 设计原则

1. **局部排序**：只对当前所在目录的子项排序，不破坏全局树结构；进入子目录后排序状态保持。
2. **稳定可预测**：同一次按键顺序在升序 / 降序 / 默认之间循环。
3. **Tree 模式友好**：树形展开时也按当前排序键对每一层兄弟节点排序。
4. **最小侵入**：复用已有的 `SortKey`/`SortState`/`Column.FmtName()` 能力，尽量不改动 `table` 组件用法。
5. **显式反馈**：列标题显示箭头，状态栏可显示当前排序列。

## 4. 交互设计

### 4.1 快捷键

| 按键 | 行为 |
|------|------|
| `s` | 循环切换排序列（Name → Languages → Code → Comments → Blanks → Total → % of Parent → 回到默认/Total） |
| `S`（`shift+s`） | 对当前排序列切换升序 / 降序 |
| `,` / `<`（可选） | 上一排序列 / 下一排序列，与 `s` 等价但方向相反 |

> 选择 `s` 的原因：现有快捷键中 `t`、`e`、`/`、`tab`、`ctrl+l`、`ctrl+w` 已被占用；`s` 与 “sort” 语义直接对应，且大小写 `s`/`S` 分别对应“切列”与“切方向”，容易记忆。

### 4.2 排序状态循环

假设当前默认排序为 **按 Total 降序**（与现有行为一致）。

- 首次按 `s`：切换到 **Name 升序**。
- 连续按 `s`：Code → Comments → Blanks → Total → % of Parent → 回到默认 / Total。
- 按 `S`：仅反转当前列的升降序。
- 当切换到某一列时，默认方向：
  - 数值列（Code、Comments、Blanks、Total、% of Parent）：默认 **降序**（大的在前）。
  - 文本列（Name、Languages）：默认 **升序**（A-Z）。

### 4.3 默认排序

为保持向后兼容，**未手动排序时仍按 Total 降序**。可以：
- 将默认 `SortState.Key` 设为 `"total"` 且 `Desc=true`；或
- 保留一个空 `SortState.Key=""` 表示“默认排序”，在列标题上只给 Total 列显示 ▼。

推荐方案：**默认 `SortState{Key: "total", Desc: true}`**，这样 UI 反馈与内部行为一致。

## 5. 数据模型变更

### 5.1 `render/column.go`

为 `SortKey` 定义常量，避免硬编码：

```go
type SortKey string

const (
    SortByNone      SortKey = ""
    SortByName      SortKey = "name"
    SortByLanguages SortKey = "languages"
    SortByCode      SortKey = "code"
    SortByComments  SortKey = "comments"
    SortByBlanks    SortKey = "blanks"
    SortByTotal     SortKey = "total"
    SortByPercent   SortKey = "percent"
)
```

`Column` 保持不变，`SortState` 保持不变，`FmtName()` 已有能力直接使用。

### 5.2 `DirModel.columns`

在 `NewDirModel` 中为可排序列绑定 `SortKey`：

```go
columns := []Column{
    {Title: ""},                               // Icon: 不参与排序
    {Title: ""},                               // Path: hidden, 不参与排序
    {Title: "Name",        SortKey: SortByName},
    {Title: "Languages",   SortKey: SortByLanguages},
    {Title: "Code",        SortKey: SortByCode},
    {Title: "Comments",    SortKey: SortByComments},
    {Title: "Blanks",      SortKey: SortByBlanks},
    {Title: "Total",       SortKey: SortByTotal},
    {Title: "% of Parent", SortKey: SortByPercent},
}
```

### 5.3 `DirModel` 新增状态

```go
type DirModel struct {
    // ... existing fields ...
    sortState SortState
}
```

并在 `NewDirModel` 初始化：

```go
dm := &DirModel{
    // ...
    sortState: SortState{Key: SortByTotal, Desc: true},
}
```

## 6. 排序实现

### 6.1 方案 A：在 `structure.Entry` 层通用化排序（推荐）

把 `Entry.SortChild()` 扩展为支持任意排序键和方向：

```go
// structure/entry.go

type SortOrder int

const (
    OrderAsc SortOrder = iota
    OrderDesc
)

type ChildSortFunc func(a, b *Entry) int

func (e *Entry) SortChildBy(fn ChildSortFunc) *Entry {
    slices.SortFunc(e.Child, fn)
    return e
}
```

在 `render` 包中根据 `SortState` 构造比较函数。这样可以：
- 保持 `structure` 包的通用性；
- 渲染层决定“当前语言过滤 / 多语言聚合后按 Code 排序”等复杂逻辑；
- 不污染 `structure` 包对 UI 排序键的感知。

### 6.2 方案 B：在 `structure.Entry` 中直接感知 SortKey

```go
func (e *Entry) SortChildByKey(key SortKey, desc bool, langFilter string) *Entry { ... }
```

不推荐：会让 `structure` 包反向依赖 `render` 包的 `SortKey` 类型，破坏分层。

### 6.3 多语言过滤下的排序语义

`updateTableData()` 对多语言过滤的展示策略是：
- `Code` / `Comments` / `Blanks` 列显示 `activeLangs[0]` 的数值；
- `Total` 列显示所有选中语言的累加值。

如果排序时按“显示值”比较，会导致不同列的排序基准不一致（例如 Code 用第一个语言、Total 用累加）。为避免这种分裂感，**排序时统一按累加值计算**：

- 无选中语言时，使用 `entry.TotalStats`；
- 单语言过滤时，使用 `entry.GetStats(activeLang)`；
- 多语言过滤时，使用 `Σ getStats(lang)`。

这样无论显示列如何取舍，排序始终反映“当前过滤条件下真实的统计大小”。

### 6.4 复用 stats 计算的 helper

为避免 `updateTableData()` 和 `buildChildComparator()` 各自维护一套语言过滤逻辑，先在 `DirModel` 上抽取两个 helper：

```go
// activeLang 返回当前单语言过滤值；多语言激活或“All”时返回 ""
func (dm *DirModel) activeLang() string {
    if dm.useMultiLangFilter() {
        return ""
    }
    if dm.langFilterIdx > -1 && dm.langFilterIdx < len(dm.languages) {
        return dm.languages[dm.langFilterIdx]
    }
    return ""
}

func (dm *DirModel) useMultiLangFilter() bool {
    if dm.selectedLangs == nil {
        return false
    }
    count := 0
    for _, lang := range dm.languages {
        if dm.selectedLangs[lang] {
            count++
        }
    }
    return count > 0
}

// comparableStats 返回排序和显示应使用统一的 CodeStats
func (dm *DirModel) comparableStats(e *structure.Entry) structure.CodeStats {
    if !dm.useMultiLangFilter() {
        return e.GetStats(dm.activeLang())
    }
    var sum structure.CodeStats
    for _, lang := range dm.languages {
        if dm.selectedLangs[lang] {
            sum.Add(e.GetStats(lang))
        }
    }
    return sum
}
```

> `updateTableData()` 中现有的 `activeLang`、`activeLangs`、`useMulti` 局部变量应替换为调用这些 helper，保证行展示与排序来自同一条计算路径。

### 6.5 推荐比较函数实现（render 层）

```go
// render/dir_model.go

func (dm *DirModel) buildChildComparator() func(a, b *structure.Entry) int {
    key := dm.sortState.Key
    desc := dm.sortState.Desc

    cmpVal := func(a, b int64) int {
        if desc {
            return cmp.Compare(b, a)
        }
        return cmp.Compare(a, b)
    }
    cmpStr := func(a, b string) int {
        r := cmp.Compare(strings.ToLower(a), strings.ToLower(b))
        if desc {
            return -r
        }
        return r
    }

    switch key {
    case SortByName:
        return func(a, b *structure.Entry) int { return cmpStr(a.Name(), b.Name()) }
    case SortByLanguages:
        return func(a, b *structure.Entry) int {
            return cmpStr(strings.Join(a.Languages(), ", "), strings.Join(b.Languages(), ", "))
        }
    case SortByCode:
        return func(a, b *structure.Entry) int { return cmpVal(dm.comparableStats(a).Code, dm.comparableStats(b).Code) }
    case SortByComments:
        return func(a, b *structure.Entry) int { return cmpVal(dm.comparableStats(a).Comments, dm.comparableStats(b).Comments) }
    case SortByBlanks:
        return func(a, b *structure.Entry) int { return cmpVal(dm.comparableStats(a).Blanks, dm.comparableStats(b).Blanks) }
    case SortByTotal:
        return func(a, b *structure.Entry) int { return cmpVal(dm.comparableStats(a).Total(), dm.comparableStats(b).Total()) }
    case SortByPercent:
        parentTotal := dm.nav.ParentTotalLines(dm.activeLang())
        return func(a, b *structure.Entry) int {
            var pa, pb float64
            if parentTotal > 0 {
                pa = float64(dm.comparableStats(a).Total()) / float64(parentTotal)
                pb = float64(dm.comparableStats(b).Total()) / float64(parentTotal)
            }
            if desc {
                return cmp.Compare(pb, pa)
            }
            return cmp.Compare(pa, pb)
        }
    default:
        // fallback: 默认 Total 降序
        return func(a, b *structure.Entry) int { return cmpVal(a.TotalStats.Total(), b.TotalStats.Total()) }
    }
}
```

然后在 `updateTableData()` 中：

```go
// 替换原有的 dm.nav.Entry().SortChild()
dm.nav.Entry().SortChildBy(dm.buildChildComparator())
```

对于 **Tree 模式**，在递归展开每一层时也需要对 `entry.Child` 排序：

```go
if entry.IsDir && entry.Expanded {
    entry.SortChildBy(dm.buildChildComparator())
    for _, child := range entry.Child {
        addEntry(child, depth+1)
    }
}
```

> 注意：
> - Tree 模式展开后再次按排序键切换，需要重新触发 `updateTableData()`，这会重新递归整棵树并排序。
> - `dm.nav.Entry()` 可能指向用户已 navigate 进入的深层目录，其 `Child` 在 `updateTableData()` 开头已被排序；树中展开的子目录的 `Child` 则在递归处独立排序。
> - `% of Parent` 在同一父目录下分母相同，因此按百分比排序的结果等价于按 `Total` 排序。 Tree 模式中每一层都使用自己的父目录作为分母，排序仍然正确。

## 7. 列标题渲染

在 `updateTableData()` 构建 `table.Column` 时，使用 `c.FmtName(dm.sortState)`：

```go
for i, c := range dm.columns {
    columns[i] = table.Column{
        Title: c.FmtName(dm.sortState),
        Width: widths[i],
    }
}
```

这样当前排序列会自动显示 `▲` / `▼`。

### 宽度修正

加入箭头后标题宽度增加 2 个字符。当前 `maxNameWidth` 计算基于 `dm.columns[2].Title`，不会受箭头影响；但其他数值列的标题（如 `Total ▼`）可能超出预设宽度。推荐：
- 在 `FmtName()` 已经追加箭头的情况下，重新计算各列标题宽度时取 `lipgloss.Width(c.FmtName(dm.sortState))`。
- 或者为所有可排序列的标题宽度预留 +2 余量。

推荐前者，更精确。

## 8. 键绑定与帮助信息

### 8.1 `render/bindings.go` 新增

```go
const (
    // ... existing ...
    cycleSortColumn  bindingKey = "s"
    toggleSortOrder  bindingKey = "S"
)
```

并在 `dirsKeyMap` 中加入帮助项：

```go
key.NewBinding(
    key.WithKeys(cycleSortColumn.String()),
    key.WithHelp(
        bindKeyStyle.Render(cycleSortColumn.String()),
        helpDescStyle.Render(" - Cycle sort column"),
    ),
),
key.NewBinding(
    key.WithKeys(toggleSortOrder.String()),
    key.WithHelp(
        bindKeyStyle.Render(toggleSortOrder.String()),
        helpDescStyle.Render(" - Toggle sort order"),
    ),
),
```

### 8.2 `DirModel.handleKeyBindings` 处理

```go
case cycleSortColumn:
    dm.cycleSortColumn()
    dm.updateTableData()
    return nil, true
case toggleSortOrder:
    dm.toggleSortOrder()
    dm.updateTableData()
    return nil, true
```

### 8.3 排序状态切换方法

```go
func (dm *DirModel) cycleSortColumn() {
    order := []SortKey{
        SortByName,
        SortByLanguages,
        SortByCode,
        SortByComments,
        SortByBlanks,
        SortByTotal,
        SortByPercent,
    }

    // 找到当前位置
    idx := -1
    for i, k := range order {
        if k == dm.sortState.Key {
            idx = i
            break
        }
    }
    idx = (idx + 1) % len(order)
    next := order[idx]

    // 切换列时重置为默认方向
    dm.sortState = SortState{
        Key:  next,
        Desc: dm.defaultDescForKey(next),
    }
}

func (dm *DirModel) toggleSortOrder() {
    dm.sortState.Desc = !dm.sortState.Desc
}

func (dm *DirModel) defaultDescForKey(key SortKey) bool {
    switch key {
    case SortByName, SortByLanguages:
        return false // 文本默认升序
    default:
        return true // 数值默认降序
    }
}
```

## 9. 状态栏扩展（可选）

在 `dirsSummary()` 中增加当前排序信息，例如：

```go
items := []*BarItem{
    // ... existing ...
    NewBarItem("SORT", "#06ffa5", 0),
    NewBarItem(fmt.Sprintf("%s %s", dm.sortState.Key, dm.sortState.DirectionArrow()), "", 0),
}
```

或在 `SortState` 上增加辅助方法：

```go
func (ss SortState) DirectionArrow() string {
    if ss.Desc {
        return "▼"
    }
    return "▲"
}
```

这样用户无需抬头看列标题也能知道当前排序状态。

## 10. 边界情况

| 场景 | 处理 |
|------|------|
| 当前目录没有子项 | `updateTableData()` 直接返回空表，排序无副作用。 |
| 多语言过滤激活 | `comparableStats()` 返回聚合后的 CodeStats；Code/Comments/Blanks/Total 均按聚合值排序，与显示策略脱钩。 |
| 百分比列除以零 | `ParentTotalLines` 已保证至少为 1。 |
| 语言列包含多个语言 | 首期按语言字符串的字母顺序拼接后比较（简单实现）；后续可改为按语言数量或主语言排序。 |
| Tree 模式深层目录 | 每一层展开时独立排序；同一父目录下 `% of Parent` 排序等价于按 `Total` 排序。 |
| 列标题宽度不够 | 使用 `FmtName()` 后的实际宽度重新计算。 |

## 11. 实现步骤（建议顺序）

1. **定义常量**：在 `render/column.go` 中为 `SortKey` 增加常量。
2. **绑定列键**：在 `NewDirModel` 中为可排序列设置 `SortKey`。
3. **扩展 Entry 排序**：在 `structure/entry.go` 中增加 `SortChildBy(fn ChildSortFunc)`，保留原 `SortChild()` 作为兼容包装。
4. **提取语言过滤 helper**：
   - 新增 `DirModel.activeLang()` 返回当前单语言过滤值；
   - 新增 `DirModel.useMultiLangFilter()` 判断是否激活多语言过滤；
   - 新增 `DirModel.comparableStats(e)` 统一返回单语言 / 多语言聚合后的 `CodeStats`。
   - 同步在 `updateTableData()` 中用这些 helper 替换现有的局部变量 `activeLang`、`activeLangs`、`useMulti`，保证展示与排序同源。
5. **实现比较器**：在 `render/dir_model.go` 中新增 `buildChildComparator()`，基于 `comparableStats()` 处理各排序键与百分比。
6. **接入 updateTableData**：
   - 6a. 顶层排序：将 `dm.nav.Entry().SortChild()` 替换为 `dm.nav.Entry().SortChildBy(dm.buildChildComparator())`。
   - 6b. Tree 模式递归：将 `entry.SortChild()` 替换为 `entry.SortChildBy(dm.buildChildComparator())`。
7. **标题渲染**：构建 `table.Column` 时使用 `c.FmtName(dm.sortState)`，并修正宽度计算。
8. **键绑定**：新增 `s`/`S` 绑定与帮助信息。
9. **状态栏**：可选增加排序状态显示。
10. **测试**：
    - 单元测试 `SortChildBy` 对各种排序键的正确性；
    - 单元测试 `comparableStats()` 在单语言 / 多语言 / 无过滤场景下的聚合行为；
    - 手动测试 Nav 模式、Tree 模式、多语言过滤、空目录等场景。

## 12. 向后兼容性

- 默认排序仍为 **Total 降序**，与当前行为完全一致。
- 新增快捷键 `s`/`S`，不影响现有快捷键。
- `Entry.SortChild()` 可保留为 `SortChildBy(totalDescComparator)` 的别名，避免破坏其他调用者。

## 13. 待决策事项

1. **排序键循环顺序**是否可配置？（建议首期硬编码，后续可加入用户配置。）
2. **Languages 列**的排序语义：首期采用“语言名按字母排序后拼接成字符串比较”的简单方案；后续若用户反馈不够直观，可改为按语言数量、主语言（占比最高）或配置项排序。
3. 是否需要持久化最后使用的排序状态到配置文件？（建议第二期再做。）
4. 是否支持**Shift+点击列标题**排序？TUI 中鼠标支持取决于 `bubbles/table`，可先只做键盘。

---

**结论**：本方案通过复用已预留的 `SortKey`/`SortState` 模型，在 `structure` 层增加通用排序能力，在 `render` 层实现按列排序的交互与渲染，能够以较小的代码量完成按列排序功能，并保持与现有行为的兼容。
