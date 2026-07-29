# Git 集成设计方案：Diff 模式与快照对比

> 状态：设计稿
> 目标：在不改变现有全量统计体验的前提下，引入 git 作为「文件集筛选器 + 快照来源」，支持 PR/commit 影响面分析与版本间统计对比。

---

## 1. 背景与问题

当前 tokui 统计的是某个路径下某一时刻的**全量快照**（记为 S1）：

```
cmd/app.go
  ├── tree.BuildFromProvider(p, path)      // 全量扫描
  └── render.DirModel                       // Name/Lang/Code/... 固定语义
```

引入 git 后出现了两个状态 S1（base）与 S2（target），需要先回答一个语义问题：**diff 场景下"代码量"到底指什么？**

### 1.1 两个必须区分的指标

| 指标 | 来源 | 语义 | 例子 |
|---|---|---|---|
| **Churn（变更量）** | `git diff --numstat S1 S2` | 这段时间"改动了多少行" | 修改一行 = +1 -1，净变化 0 |
| **ΔStats（统计量净变化）** | 两个快照各跑一次 Provider 再相减 | 这段时间"项目变大/变复杂了多少" | ΔCode、ΔComplexity |

一个重构 PR 可能 churn 很大（+500/-500）但 ΔCode = 0。churn 回答"动了多少"，ΔStats 回答"改变了什么"，两者不可互相替代。

### 1.2 git 在架构中的角色

git **不是统计后端**，不适合实现为第三个 `provider.Provider`。它的角色是：

1. **文件集筛选器**：`git diff` 决定"统计哪些文件"；
2. **快照来源**：`git archive <ref>` 决定"统计哪个版本的代码"。

统计本身仍由现有 Provider（tokei/scc）完成。

---

## 2. 功能场景

### 场景 A：PR / commit range 影响面分析（Diff 模式，默认推荐）

```bash
tokui --diff main...HEAD      # 当前分支相对 main 的改动
tokui --diff HEAD~3           # 最近 3 个 commit
tokui --diff HEAD             # 工作区未提交的改动
tokui --diff                  # 同上，裸 --diff 等价于 --diff HEAD（同 git diff）
```

树中只包含 diff 涉及的文件，目录照常聚合。回答："这次改动动了多少代码、集中在哪些目录/语言、哪些文件是大头。"

### 场景 B：版本间统计对比（Compare 模式）

```bash
tokui --compare v1.0..v2.0    # 两个 tag 的统计净变化
tokui --compare main..HEAD
```

对 S1（`git archive` 到临时目录）和 S2 各跑一次 Provider，展示每个文件/目录/语言的 ΔCode / ΔComplexity。回答："这个区间让代码量/复杂度净增了多少。"

### 场景 C：历史版本单快照浏览（`--ref`）

```bash
tokui --ref v1.0               # 查看某个 tag/commit 的完整代码构成
```

形态与全量模式完全一致，只是数据源是 `git archive <ref>` 解包到临时目录的快照。它是 Compare 模式的简化形态（只看一侧），实现几乎零成本（复用 `gitx.Archive` + 现有 `BuildFromProvider`）。

### 场景 D：文件级历史预览（`git show` 的直接场景）

Diff / Compare 模式下，预览（`Enter`）默认显示 S2 内容；按 `v` 切换到该文件的 S1 版本（`git show S1:<path>`），再按切回。已删除文件无 S2 内容，直接预览 S1 版本。回答："这个文件改动前长什么样。"

注意与场景 C 的工具分工：**整树历史快照用 `git archive`**（Provider 需要真实文件系统），**单文件历史内容用 `git show`**（按需取一个文件，轻量）。不对整棵树逐文件调 `git show`。

### 场景 E：非 git 环境降级

- 目标路径不是 git 仓库：报明确错误 `not a git repository: <path>`，提示去掉 `--diff/--compare` 使用全量模式。
- `git` 二进制不在 PATH：报明确错误并附安装提示（复用现有 provider 缺失时的错误处理模式，见 `cmd/app.go:196`）。

### 明确不做的场景

- **不做** blame / log / commit 浏览等 lazygit 类功能，避免偏离"代码统计可视化"定位。
- **不做** 文件级 diff 内容展示（`git diff` 逐行视图），预览仍展示 S2 文件全文。

---

## 3. 展示形态

### 3.1 原则：以 S2 为主体，变更作为增量列

不做每个指标的 S1/S2 双列并排（列数爆炸、绝大多数单元格重复）。Diff 模式显示 churn 三列 + S2 现值列；Compare 模式用 `旧 → 新` 单对比列 + Δ 排序列。

### 3.2 Diff 模式列布局

假设 `git diff --numstat` 输出：

```
+120  -45   render/dir_model.go
+80   -10   render/column.go
+200  -0    provider/git/provider.go   (新增)
-0    -150  filter/list.go             (重构删减)
+15   -3    cmd/app.go
```

主视图：

```
  Name                  Lang        +      -      Δ      %      Code    Total
▸ render               Go        +200   -55   +145   43.3 %   4,230    6,120
▸ provider/git         Go        +200     0   +200   37.7 %     480      690
▾ filter               Go           0  -150   -150   16.0 %     310      460
    list.go            Go           0  -150   -150   16.0 %     310      460
  cmd/app.go           Go         +15    -3    +12    3.0 %     210      340
```

- `+` / `-` / `Δ`：churn 三列，`Δ` 为净变化。默认按 `|Δ|` 降序，大头变更永远在最上。
- `%`：churn（`+` 加 `-`）占当前目录总 churn 的百分比，用于快速定位改动集中的目录/文件。
- `Code` / `Total`：S2 侧现值（对 S2 跑一遍现有 Provider，通常就是当前工作区，无额外成本），用于判断"这个改了 +200 的文件现在总共多大"。
- 目录行 = 子文件之和，聚合语义与现有一致。
- 新增/删除文件：`0 → x` 或 `x → 0` 体现在 Code 列（新增文件 S1 无值，churn 列已自然表达）。
- 未变更文件不进树；按 `a` 切换显示全量（churn 为 0 的行灰色显示，作为导航上下文）。

### 3.3 Compare 模式列布局

```
  Name                   Code (S1 → S2)     ΔCode    ΔCmplx
▸ render                 4,085 → 4,230      +145      +12
  filter/list.go           460 → 310        -150       -8
  provider/git/new.go        0 → 480        +480       +5
  legacy/dead.go           700 → 0          -700        0    (已删除)
```

- 一列完成对比，Δ 列负责排序。
- 已删除文件标 `(已删除)`；新增文件 `0 → x`。
- 未变更文件 Δ = 0，默认折叠隐藏，按 `a` 切换全量（此时等价于现有 S2 全量视图）。

### 3.4 状态栏

`dirsSummary()` 增加一行汇总，Diff 模式：

```
main...HEAD · 5 files changed · +415 / -208 · Δ +207 · tokei 12.1
```

Compare 模式：

```
v1.0..v2.0 · 23 files changed · ΔCode +1,204 · ΔCmplx +31 · scc 3.x
```

### 3.5 与现有交互的复用

- **排序**：`s` 循环键扩展为 `Name → + → - → Δ → % → Code → Total`（Compare 模式为 `Name → ΔCode → ΔCmplx`），`S` 反转方向，逻辑不变。
- **树模式 / treemap**：不做特殊处理，churn/Δ 参与块大小计算（treemap 按 |Δ| 或 churn 占比）。
- **语言过滤**（`Tab` / `Ctrl+L`）：Diff 模式语言归属来自 S2 侧 Provider 结果（见 4.3），行为与现有一致；已删除文件语言按扩展名映射。
- **文件预览**：`Enter` 预览 S2 文件全文（现有逻辑）；按 `v` 切换为该文件的 S1 版本（`git show S1:<path>`），再按切回；已删除文件直接预览 S1 版本。
- **窄屏适配**：复用现有隐藏策略，优先级 `Name > + > - > Δ > % > Code > Total`（从右往左隐藏，`%` 在 `Total` 之后、`Code` 之前隐藏）。

---

## 4. 实现方式

### 4.1 新增 `gitx` 包

只做 git 命令的封装，零新增第三方依赖（`os/exec` 调系统 git）：

```go
package gitx

type ChangeKind int

const (
    Modified ChangeKind = iota
    Added
    Deleted
    Renamed
)

type FileChange struct {
    Path    string     // S2 侧相对仓库根的路径；Deleted 时为 S1 侧路径
    OldPath string     // 仅 Renamed 时有值
    Added   int64
    Deleted int64
    Kind    ChangeKind
}

// Numstat 解析 git diff --numstat -M 的输出。
func Numstat(repoRoot, rangeSpec string) ([]FileChange, error)

// RepoRoot 返回 path 所属仓库根目录；非 git 仓库返回错误。
func RepoRoot(path string) (string, error)

// Archive 将 ref 的工作树解包到临时目录，返回目录路径与清理函数。
func Archive(ref string) (dir string, cleanup func(), err error)

// ShowFile 返回 ref:path 的文件内容（用于 S1 版本与已删除文件预览）。
func ShowFile(repoRoot, ref, path string) ([]byte, error)
```

边界处理：

- **二进制文件**：numstat 输出 `- -`，记 Added/Deleted = 0 并保留条目（仍值得展示"这个二进制变了"）。
- **rename**：用 `-M` 开启重命名检测，`numstat` 的 `old => new` 格式解析为一条 `Renamed` 记录，Path 取新路径。
- **submodule**：numstat 中显示为 `- -` 且非文件，跳过。
- **空 diff**：range 无差异时展示空树 + 状态栏 `no changes in <range>`，不报错。

### 4.2 数据模型：Entry 附加 Change

`structure.CodeStats` 不动（仍是 S2 快照统计），变更信息作为独立字段挂在 `Entry` 上：

```go
package structure

type Change struct {
    Added   int64
    Deleted int64
    Kind    gitx.ChangeKind
    // Compare 模式专用：
    PrevCode       int64  // S1 侧 Code
    PrevComplexity int64  // S1 侧 Complexity
}

func (c Change) Delta() int64 { return c.Added - c.Deleted }
```

- `AggregateStats` 同步向上聚合 `Change`（Added/Deleted 求和；Prev* 求和）。
- 语言过滤时 Change 随 `StatsByLang` 走同样的过滤路径——Change 挂在文件 Entry 上、文件按语言归类，与现有 `StatsByLang` 构建顺序一致。

### 4.3 树的构建入口

新增两个构建函数，`BuildFromProvider*` 保持不变：

```go
// Diff 模式：changes 过滤文件集 + S2 侧 Provider 结果提供语言与现值。
func (t *Tree) BuildFromDiff(changes []gitx.FileChange, s2 provider.Result, repoRoot string) error

// Compare 模式：两份快照结果按路径 join，计算 Prev*/Δ。
func (t *Tree) BuildFromCompare(s1, s2 provider.Result, root string) error
```

Diff 模式流程：

1. `gitx.RepoRoot(path)` 校验并定位仓库根；
2. `gitx.Numstat(root, rangeSpec)` 得到变更集；
3. 对 S2 跑现有 Provider（range 含工作区时分析工作区，否则 `gitx.Archive(S2)` 到临时目录后分析）；
4. 以变更集路径为骨架建树，从 S2 结果中取每个文件的 `CodeStats`（含语言）；已删除文件无 S2 值，语言按扩展名映射，CodeStats 置零、Kind=Deleted。

Compare 模式流程：

1. `gitx.Archive(S1)` 解包旧快照；S2 是 `HEAD` 或工作区时直接用现有路径，否则同样 Archive；
2. 两份 `provider.Result` 按归一化路径 join（复用 `normalizePath`）；
3. 并集建树，每文件填 S2 CodeStats + Change{PrevCode, PrevComplexity}。

### 4.4 CLI 入口

`cmd/app.go` 新增三个互斥标志：

```go
appCmd.Flags().String("diff", "", `Show churn for a git range, e.g. "main...HEAD" or "HEAD~3".`)
appCmd.Flags().String("compare", "", `Compare statistics between two git refs, e.g. "v1.0..v2.0".`)
appCmd.Flags().String("ref", "", `Show statistics for a single git ref snapshot, e.g. "v1.0".`)
appCmd.MarkFlagsMutuallyExclusive("diff", "compare", "ref")
```

执行分支（在 pipe 判断之前）：

```go
switch {
case diffRange != "":
    err = runDiffMode(tree, p, root, diffRange)
case compareRange != "":
    err = runCompareMode(tree, p, root, compareRange)
case refName != "":
    err = runRefMode(tree, p, root, refName) // gitx.Archive(ref) + 现有 BuildFromProvider
case isPipe:
    // 现有逻辑
default:
    // 现有逻辑
}
```

- `--diff/--compare/--ref` 与 pipe 模式互斥（stdin 非 tty 且带这三个标志时报错提示）。
- range 语法直接透传给 `git diff`（支持 `..`、`...`、单个 commit、`HEAD`），不在 CLI 层做语法校验，git 报错原样透传并附示例。
- 视图层需要知道当前模式与 range 字符串（状态栏展示），`initViewModel` 增加一个 `ModeInfo` 参数：

```go
type ModeInfo struct {
    Kind  ModeKind // Full | Diff | Compare | Ref
    Range string   // diff/compare 的 range，或 ref 名
}
```

### 4.5 渲染层

- **Capability 扩展**：`provider.CapChurn`、`provider.CapDelta` 两个能力位；git 模式下由 CLI 层合成一个带这两个能力的 `provider.Info` 传给 `NewDirModel`，动态列逻辑沿用 Provider 方案既有机制。
- **SortKey 扩展**（`render/column.go`）：`SortByAdded`、`SortByDeleted`、`SortByDelta`；Compare 模式复用 `SortByDelta` + `SortByComplexity`（Δ 语义）。
- **列定义**（`render/dir_model.go`）：按 `ModeInfo.Kind` + Capabilities 构建：
  - Diff：`icon, path, name, lang, added(+), deleted(-), delta(Δ), percent(%), code, total`
  - Compare：`icon, path, name, codeCompare(S1 → S2), deltaCode, deltaComplexity`
  - `s` 的排序循环键表随列定义动态生成。
- **`a` 键**：切换"仅变更 / 全量"。实现为树的过滤器（类似现有语言过滤），而非重建树。
- **状态栏**：`dirsSummary()` 按 `ModeInfo` 渲染 3.4 节的汇总行。
- **预览（`git show` 场景）**：Diff / Compare 模式下预览默认显示 S2 内容，按 `v` 切换为 S1 版本（`gitx.ShowFile(repoRoot, S1, path)`），再按切回；已删除文件直接预览 S1 版本。预览层持有 `repoRoot` 与 S1 ref，**按需调用** `ShowFile`（不预取，避免为大文件浪费 IO）。预览覆盖层标题栏显示当前版本标记（`S1: v1.0` / `S2: worktree`）。

### 4.6 语言归属（Diff 模式已删除文件）

S2 结果中没有已删除文件，无法从 Provider 拿语言。实现一个小的扩展名→语言映射（覆盖常见后缀即可，未知归 `Other`），仅用于已删除文件；保持简单，不引入 tokei 的语言数据库。

---

## 5. 方案对比

| 方案 | 说明 | 评价 |
|---|---|---|
| **A. gitx 封装 + Tree 层 join（推荐）** | git 产出变更集/快照，Provider 产出统计，Tree 层合并 | 角色清晰、复用全部现有渲染与聚合；改动面中等 |
| B. git 实现为 Provider | 把 churn 伪装成 Code/Comments | 语义错乱（churn ≠ code），排序/聚合全要特判 |
| C. 只做 numstat 不跑 Provider | 纯 churn 视图，无语言/现值列 | 实现最简，但丢失语言过滤与预览两大核心体验 |
| D. go-git 库替代 exec git | 不依赖系统 git | 引入重依赖；archive/diff 边缘行为与 CLI git 有差异；违背项目零依赖取向 |

---

## 6. 实现步骤

1. 新增 `gitx` 包：`RepoRoot` / `Numstat`（含 rename、二进制解析）+ 单元测试（用临时 git 仓库做 fixture）。
2. `structure.Change` 与 `AggregateStats` 聚合扩展；`BuildFromDiff` 实现。
3. CLI 增加 `--diff`，串通 Diff 模式全链路（含非 git 目录降级错误）。
4. 渲染层：`CapChurn` 能力位、Diff 列定义、三个新 SortKey、状态栏汇总行。
5. `a` 键全量/仅变更切换。
6. 已删除文件的扩展名语言映射。
7. 文件级历史预览：`gitx.ShowFile` + 预览层 `v` 键切换 S1/S2 版本（含已删除文件直接预览 S1）。
8. `gitx.Archive` + `--ref` 单快照模式（复用现有 `BuildFromProvider`，最小落地）。
9. `BuildFromCompare` + `--compare` + Compare 列定义（`CapDelta`）。
10. 补充测试：
   - numstat 解析（rename、二进制、空 diff、submodule）。
   - `BuildFromDiff` / `BuildFromCompare` 树结构与聚合值。
   - 目录级 Change 聚合与语言过滤交互。
   - 动态列在四种模式 × 不同终端宽度下的渲染。
   - `ShowFile` 预览切换（修改/删除/新增文件）。
   - 非 git 目录、git 缺失、非法 range/ref 的错误提示。
   - `--diff/--compare/--ref` 与 pipe、`--tree/--treemap` 的组合行为。

阶段划分：步骤 1–7 为第一阶段（Diff 模式 + 历史预览），步骤 8–9 为第二阶段（快照浏览与对比），可独立发布。

---

## 7. 风险与应对

| 风险 | 应对 |
|---|---|
| **churn 与 Δ 语义混淆** | 文档与列头明确区分；状态栏始终显示 range，让用户知道在看什么。 |
| **range 语法多样**（`..`/`...`/commit/HEAD） | 不自行解析，透传 git；错误原样展示并附示例。 |
| **S2 是历史 ref 时工作区不可用** | Diff 模式统一走 `gitx.Archive(S2)` 解包临时目录再分析，保证可预览；range 为 `HEAD`（含工作区改动）时直接分析工作区。 |
| **临时目录清理** | `Archive` 返回 cleanup 函数，`cmd` 层 defer 调用；程序退出前清理。SIGKILL 时 defer 覆盖不到，临时目录会残留（固有局限，由系统临时目录的定期清理兜底）。 |
| **大仓库双跑成本** | Compare 模式为显式开关；Diff 模式只分析 S2 一侧，成本与现有全量一致。 |
| **Compare 模式 S2=HEAD 的语义** | `--compare v1..HEAD` 的 S2 分析的是工作区（含未提交改动），这是 §4.3 的既定行为；状态栏将 range 显示为 `v1..HEAD (worktree)` 以明确标注。 |
| **rename 路径归属** | `-M` 检测后按新路径归属，churn 记在该路径；Compare 按路径 join 时 rename 会表现为 删+增，可接受，不特殊处理。 |
| **已删除文件无语言/无 S2 统计** | 扩展名映射归语言；统计置零、Kind=Deleted，预览走 `git show`。 |

---

## 8. 结论

git 集成的正确形态是 **gitx（变更集/快照来源）+ 现有 Provider（统计）+ Tree 层 join**，而不是第三个 Provider。git 三个命令各司其职：`git diff --numstat` 产出变更集（Diff 模式），`git archive` 产出整树快照（`--ref` 与 Compare 模式），`git show` 按需取单文件历史内容（预览 `v` 键切换）。第一阶段 Diff 模式（`--diff`）覆盖 code review 这个最高频场景，展示上以 S2 为主体、churn 三列做增量；第二阶段补快照浏览（`--ref`）与净变化对比（`--compare`）。各模式共享同一棵树与同一套交互，渲染层通过既有 Capability 机制动态出列，侵入可控。
