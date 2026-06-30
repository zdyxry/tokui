# Provider 抽象设计方案：支持 scc 复杂度统计

> 状态：设计稿（已根据 review 意见修订）  
> 目标：在不破坏现有 tokei 体验的前提下，引入 scc 作为可选统计后端，并支持其复杂度（Complexity）等扩展指标。

---

## 1. 背景与问题

当前 `tokui` 只支持代码行数相关统计（Code / Comments / Blanks / Total），统计后端固定为外部 `tokei` 二进制：

```
cmd/app.go
  ├── structure.Tree.BuildFromTokei / BuildFromStdin
  │     └── tokei.Analyze / AnalyzeFromStdin   // 调外部二进制 + 解析 JSON
  ├── render.Navigation
  └── render.DirModel
```

如果引入 `scc`（github.com/boyter/scc）作为库，可以获得复杂度（Complexity）、ULOC 等指标。但 scc 本身也能统计行数，若与 tokei 同时跑两遍，会造成 IO 和计算重复。

因此需要把统计后端抽象为 **Provider**：

- `tokei` 是一种 Provider，保留默认行为与现有 pipe 模式。
- `scc` 是另一种 Provider，在支持复杂度时动态展示相关列。
- 未来可继续扩展其他后端（如 lizard、gocloc 等）。

---

## 2. 关于“函数统计”的说明

`scc` 官方输出字段为 `Files / Lines / Blanks / Comments / Code / Complexity / ULOC / COCOMO`，**并不提供函数个数或函数级复杂度**。其 Complexity 是“文件级分支/循环计数”的近似 cyclomatic complexity。

因此本方案**第一阶段先引入 Complexity**，函数统计若后续确实需要，再作为独立 Provider 能力接入（如 lizard、tree-sitter 等）。

---

## 3. Provider 接口设计

新增 `provider` 包：

```go
package provider

type Capability uint

const (
    CapLines Capability = 1 << iota
    CapComplexity
)

type Info struct {
    Name         string
    Version      string
    Capabilities Capability
}

type FileStats struct {
    Path       string
    Language   string
    Code       int64
    Comments   int64
    Blanks     int64
    Complexity int64   // 仅当 CapComplexity 时有效
}

type Result struct {
    Files []FileStats
}

type Provider interface {
    Info() Info
    Analyze(path string) (Result, error)
    ParseStdin(data []byte) (Result, error) // 支持 pipe 模式
}
```

---

## 4. 现有 tokei 改造

将 `tokei/tokei.go` 改造为 `provider.Provider` 实现，保留原有 JSON 解析逻辑：

```go
package tokei

type TokeiProvider struct {
    mu       sync.Mutex
    version  string
    versionErr error
}

func New() *TokeiProvider { return &TokeiProvider{} }

func (p *TokeiProvider) Info() provider.Info {
    return provider.Info{
        Name:         "tokei",
        Version:      p.version(), // lazy-init，失败时返回 "unknown"
        Capabilities: provider.CapLines,
    }
}

func (p *TokeiProvider) Analyze(path string) (provider.Result, error) { ... }
func (p *TokeiProvider) ParseStdin(data []byte) (provider.Result, error) { ... }
```

`Info().Version` 对 tokei 采用 **lazy-init**：首次调用 `Info()` 时执行 `tokei --version` 并缓存，避免在 Provider 构造阶段就依赖二进制路径解析。若二进制不可用，版本显示为 `"unknown"`。

---

## 5. 新增 scc Provider

新增 `provider/scc` 包，使用 `github.com/boyter/scc/v3/processor` 作为库调用。

### 5.1 Go 版本

`scc/v3` 的 `go.mod` 要求 Go ≥ 1.25.2（v3.7.0 为 `go 1.25.2`，master 为 `go 1.26.4`）。当前项目为 `go 1.24.0`，**需要升级项目 Go 版本至 ≥ 1.25.2**。本方案直接升级，不考虑 fork 或 exec 调用等绕开方式。

### 5.2 全局初始化（sync.Once）

scc 的 `processor` 包依赖 `ProcessConstants()` 全局初始化。为避免重复初始化与并发问题，在 `SCCProvider` 中用 `sync.Once` 包装：

```go
type SCCProvider struct {
    initOnce sync.Once
    initErr  error
}

func (p *SCCProvider) init() {
    p.initOnce.Do(func() {
        p.initErr = processor.ProcessConstants()
    })
}

func (p *SCCProvider) Analyze(path string) (provider.Result, error) {
    p.init()
    if p.initErr != nil {
        return provider.Result{}, p.initErr
    }
    // ...
}
```

### 5.3 能力声明

```go
func (p *SCCProvider) Info() provider.Info {
    return provider.Info{
        Name:         "scc",
        Version:      "3.x", // 可读取 processor 内建版本或硬编码
        Capabilities: provider.CapLines | provider.CapComplexity,
    }
}
```

---

## 6. 数据模型改造

### 6.1 扩展 CodeStats

为支持目录级的“最复杂文件”展示，除求和外增加 `MaxComplexity`：

```go
package structure

type CodeStats struct {
    Code         int64
    Comments     int64
    Blanks       int64
    Complexity   int64   // 当前范围（文件/语言/目录）的复杂度总和
    MaxComplexity int64  // 当前范围内单文件最大复杂度（目录/多语言聚合时有用）
}

func (cs CodeStats) Total() int64 {
    return cs.Code + cs.Comments + cs.Blanks
}

func (cs *CodeStats) Add(other CodeStats) {
    cs.Code += other.Code
    cs.Comments += other.Comments
    cs.Blanks += other.Blanks
    cs.Complexity += other.Complexity
    if other.MaxComplexity > cs.MaxComplexity {
        cs.MaxComplexity = other.MaxComplexity
    }
}
```

### 6.2 Complexity 聚合语义

- **文件级**：`Complexity` 与 `MaxComplexity` 相同，均为 scc 返回的文件复杂度。
- **目录/语言级**：`Complexity` 做**求和**（最直观，反映该范围总体复杂度）；`MaxComplexity` 取子节点最大值，用于定位高风险文件。
- 表格默认显示 `Complexity`（总和），用户可通过排序发现最复杂文件；后续可考虑在文件 preview 或详情中展示 `MaxComplexity`。

### 6.3 Tree 构建入口解耦

`structure/tree.go` 不再直接依赖 tokei：

```go
func (t *Tree) BuildFromProvider(p provider.Provider, path string) error
func (t *Tree) BuildFromProviderResult(result provider.Result, root string) error
```

`BuildFromProviderResult` 接受已解析的 `provider.Result`，便于 pipe 模式下先读取 stdin、再做格式嗅探与解析。现有 `BuildFromTokei` / `BuildFromStdin` 由 Provider 选择替代。

`AggregateStats` 需要把 `Complexity`、`MaxComplexity` 同步向上聚合。

---

## 7. 路径归一化

当前 `tree.go` 的 `BuildFromTokei` 中包含约 50 行路径归一化逻辑，用于处理 tokei 可能返回绝对路径或相对路径的情况。切换到 scc 后，路径格式可能不同。

### 7.1 设计原则

- **统一在 Tree 层处理**：Provider 返回的 `FileStats.Path` 应为原始路径；Tree 负责将其转换为相对于扫描根目录的相对路径。
- **复用现有逻辑**：将现有 `TrimPrefix(absPath)/TrimPrefix("/")/TrimPrefix("./")` 逻辑提取为独立函数 `normalizePath(root, raw string) string`，tokei 与 scc 共用。
- **测试先行**：实现阶段先对 scc 返回路径格式做对比测试，确保两种 Provider 最终生成的相对路径一致。

### 7.2 建议函数

```go
func normalizePath(root, raw string) string {
    raw = filepath.ToSlash(raw)
    root = filepath.ToSlash(root)

    rel := strings.TrimPrefix(raw, root)
    rel = strings.TrimPrefix(rel, "/")
    rel = strings.TrimPrefix(rel, "./")
    return rel
}
```

若 scc 返回的是相对路径，则 `root` 前缀不会匹配，直接返回原值即可。

---

## 8. 渲染层改造

### 8.1 动态列

当前 `render/dir_model.go` 中列定义硬编码：

```go
columns := []Column{
    {Title: ""},                                    // Icon
    {Title: ""},                                    // Path (hidden)
    {Title: "Name", SortKey: SortByName},
    {Title: "Languages", SortKey: SortByLanguages},
    {Title: "Code", SortKey: SortByCode},
    {Title: "Comments", SortKey: SortByComments},
    {Title: "Blanks", SortKey: SortByBlanks},
    {Title: "Total", SortKey: SortByTotal},
    {Title: "% of Parent", SortKey: SortByPercent},
}
```

改为根据 `Provider.Info().Capabilities` 动态构建：

```go
var columns []Column
columns = append(columns, iconCol, pathCol, nameCol, langsCol)
columns = append(columns, codeCol, commentsCol, blanksCol, totalCol, percentCol)

if caps&provider.CapComplexity != 0 {
    columns = append(columns, complexityCol)
}
```

### 8.2 排序键扩展

`render/column.go` 新增：

```go
const (
    SortByComplexity SortKey = "complexity"
)
```

### 8.3 窄屏适配

新增列会挤压 `Name` 列空间，加入按终端宽度自动隐藏策略：

- 宽度 `< 80`：隐藏 `Complexity` 列。
- 宽度 `< 60`：保持核心列（Name / Code / Total）可见，必要时隐藏 Languages / Comments / Blanks。

隐藏逻辑在 `updateTableData()` 构建列之前执行，并优先保留用户当前排序列不被隐藏（若正在按 Complexity 排序，则 Complexity 不被隐藏）。

### 8.4 NewDirModel 签名

`NewDirModel` 不再接收 `tokeiVersion string`，改为接收 `provider.Info`：

```go
func NewDirModel(nav *Navigation, info provider.Info, treeMode, treemapMode bool) *DirModel
```

状态栏 `dirsSummary()` 中写死的 `tokei %s` 改为：

```go
items = append(items, NewBarItem(fmt.Sprintf("%s %s", info.Name, info.Version), "#8338ec", 0))
```

---

## 9. CLI 入口与 stdin 兼容性

### 9.1 --provider 标志

`cmd/app.go` 增加：

```go
appCmd.PersistentFlags().String("provider", "tokei", "Stats provider: tokei|scc")
```

执行逻辑：

```go
p, err := selectProvider(providerName)
// ...

tree := structure.NewTree(nil)
if isPipe {
    data, _ := io.ReadAll(os.Stdin)
    result, used, _ := parseStdinWithProvider(p, data)
    err = tree.BuildFromProviderResult(result, used.Info().Name)
} else {
    err = tree.BuildFromProvider(p, root)
}
```

### 9.2 pipe 模式：格式自动识别

当前用户习惯：

```
tokei -o json . | tokui
```

切换 provider 后，pipe 侧命令也需要切换，容易产生难以理解的 parse 错误。本方案采用 **stdin 格式嗅探（format sniffing）**：

1. 从 stdin 读取全部内容。
2. 先尝试按 tokei JSON 解析；成功则使用 tokei Provider 结果。
3. 失败则尝试按 scc JSON 解析；成功则使用 scc Provider 结果。
4. 都失败时返回明确错误：
   - 若用户显式 `--provider=scc` 但输入是 tokei JSON：提示 `"stdin looks like tokei JSON, but --provider=scc expects scc JSON; omit --provider to auto-detect"`。
   - 若无法识别：提示支持的两种 pipe 命令示例。

实现上在 `cmd/app.go` 增加 `parseStdinWithProvider`：

```go
func parseStdinWithProvider(p provider.Provider, data []byte) (provider.Result, provider.Provider, error)
```

先尝试用户指定的 Provider；若失败且用户未显式选择非默认 Provider，则回退尝试其他已知 Provider。

---

## 10. Complexity 与语言过滤的交互

scc 的 Complexity 是文件级指标，但 `FileStats` 已包含 `Language` 字段，因此在构建 `StatsByLang` 时可以把该文件的 Complexity 归入其对应语言：

```go
fileStats[relativePath][lang] = structure.CodeStats{
    Code:       r.Code,
    Comments:   r.Comments,
    Blanks:     r.Blanks,
    Complexity: r.Complexity,
    MaxComplexity: r.Complexity,
}
```

这样：

- 单语言过滤时，Complexity 列显示该语言的复杂度总和，与 Code 列行为一致。
- 多语言过滤时，Complexity 显示选中语言的复杂度总和。
- `All` 时显示全部语言复杂度总和。

对于 tokei Provider（`CapLines`  only），Complexity 列不显示，不存在交互问题。

---

## 11. 方案对比

| 方案 | 说明 | 评价 |
|---|---|---|
| **A. Provider 抽象（推荐）** | tokei / scc 各实现一个 Provider | 保留兼容、可扩展、结构清晰；改动面中等 |
| **B. 完全替换为 scc** | 删除 tokei，只用 scc | 代码最简洁，但破坏 pipe 模式，且语言识别/速度与 tokei 有差异 |
| **C. tokei 行数 + scc 复杂度** | 双跑两个工具再合并 | 实现简单，但扫描两遍 IO，慢且重复 |
| **D. scc 作为 `--complexity` 开关** | 默认 tokei，开关触发用 scc 重跑 | 比 Provider 简单，但语义是“重跑”，不能混合两个源 |

---

## 12. 实现步骤

1. **技术验证**：`go get github.com/boyter/scc/v3@latest`，确认实际版本要求与编译可行性。
2. **升级 Go 版本**至 ≥ 1.25.2。
3. 新建 `provider` 包，定义接口与能力常量；在 `cmd/app.go` 实现 `parseStdinWithProvider` 完成 stdin 自动识别。
4. 改造 `tokei/tokei.go` 为 `TokeiProvider`，实现 `provider.Provider`，版本 lazy-init。
5. 扩展 `structure.CodeStats`（含 `MaxComplexity`）与 `Tree` 构建入口；统一路径归一化函数。
6. **渲染状态栏先行**：修改 `NewDirModel` 接收 `provider.Info`，替换状态栏中的 `tokei %s`。
7. **渲染动态列**：按 `Capabilities` 动态生成列与排序键，加入窄屏隐藏策略。
8. 新增 `provider/scc` 包，实现 scc 库调用（`sync.Once` 初始化、结果映射）。
9. `cmd/app.go` 增加 `--provider` 标志、`selectProvider` 与 `parseStdinWithProvider` 逻辑。
10. 补充测试：
    - `TokeiProvider` / `SCCProvider` 接口符合性测试。
    - SCCProvider 结果映射到 `provider.FileStats`。
    - `AggregateStats` 正确处理 Complexity / MaxComplexity。
    - 动态列在不同 `Capabilities` 与不同终端宽度下的渲染。
    - 路径归一化对 tokei 与 scc 路径的一致性。
    - pipe 模式自动识别与错误提示。
    - `--provider` 切换的端到端行为。

---

## 13. 风险与应对措施

| 风险 | 应对措施 |
|---|---|
| **Go 版本门槛** | 直接升级项目 Go 版本到 ≥ 1.25.2；实现前先 `go get` 验证。 |
| **stdin 格式耦合** | pipe 模式支持 tokei/scc JSON 自动识别，并给出清晰错误提示。 |
| **路径归一化差异** | 统一 `normalizePath` 函数；实现阶段对比测试两种 Provider 的路径输出。 |
| **scc 全局初始化** | 用 `sync.Once` 包装 `ProcessConstants()`。 |
| **Complexity 聚合语义** | 目录级 `Complexity` 求和，`MaxComplexity` 取最大；文档中明确。 |
| **语言过滤交互** | 按文件语言把 Complexity 归入 `StatsByLang`，Complexity 随语言过滤变化。 |
| **窄屏列溢出** | 按终端宽度自动隐藏低优先级列，优先保证 Name / Code / Total 可读。 |
| **版本获取时机** | tokei 版本 lazy-init 并缓存，失败显示 `"unknown"`。 |

---

## 14. 结论

采用 **Provider 抽象** 是正确方向：`tokei` 与 `scc` 各自作为独立后端，由 UI 根据能力动态展示列。默认保留 tokei 以保证向后兼容，`scc` 作为可选后端提供复杂度、字节数等扩展指标。

本方案已针对 stdin 兼容性、路径归一化、全局状态、Complexity 聚合语义、语言过滤交互、窄屏适配等关键风险点给出明确设计，可直接进入实现阶段。
