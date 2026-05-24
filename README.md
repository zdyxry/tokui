# 📊 Tokui

[![Build](https://github.com/zdyxry/tokui/actions/workflows/build.yml/badge.svg)](https://github.com/zdyxry/tokui/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/zdyxry/tokui)](https://goreportcard.com/report/github.com/zdyxry/tokui)

**Tokui** is a high-performance, cross-platform command-line tool for visualizing and exploring your code statistics. It integrates with the powerful code statistics engine [tokei](https://github.com/XAMPPRocky/tokei) to present code line count metrics through a responsive, keyboard-driven Terminal User Interface (TUI), helping you to quickly analyze code composition and understand your project's structure.

> **Project Origin**
>
> This project is a fork of the excellent disk space analyzer [noxdir](https://github.com/crumbyte/noxdir), and was heavily modified and refactored by a **Large Language Model (LLM)**, transforming its functionality from a disk analyzer into a code statistics visualizer.

## 📸 Previews

![Demo](./assets/demo.gif)

## ✨ Features

- **Interactive Terminal UI**: Navigate through your project's directory structure with ease using your keyboard.
- **Deep Tokei Integration**: Leverages the powerful analysis capabilities of `tokei` for accurate code statistics.
- **Detailed Data Analysis**: Displays lines of code, comments, blanks, and total lines, categorized by language.
- **File Preview**: Press `Enter` on any file to instantly preview its contents in a scrollable overlay window.
- **Visual Charts**: Toggle a pie chart with `Ctrl+w` to intuitively visualize the proportion of each language.
- **Dynamic Filtering and Searching**:
  - Quickly search by file name (`/`).
  - Filter statistics by a specific language (`Tab`), or select multiple languages for combined filtering (`Ctrl+L`).
  - Flexible language selection overlay: Press `Ctrl+L` to open a multi-select menu for languages.
- **Tree Mode**: Toggle between navigation mode (enter directories) and tree mode (expand/collapse directories inline) with `t`.
- **Zero-Dependency Release**: Pre-built binaries bundle [tokei](https://github.com/XAMPPRocky/tokei) internally—no separate installation required.
- **High-Performance & Lightweight**: Written in Go, it compiles to a single binary.
- **Privacy-Focused**: Runs entirely locally. No telemetry or data uploads, ever.


#### Main View
A clean table showing code statistics for files and subdirectories in the current path.
```
┌─ 📊 Code Statistics ───────────────────────────────────────────────────────┐
│  ICON  NAME                    LANGUAGES        CODE      TOTAL         %  │
│  📂    internal                Go, ...          15,021    18,345    65.3%  │
│  💻    main.go                 Go               850       1,010     3.6%   │
│  📜    README.md               Markdown         120       150       0.5%   │
│  ...                                                                       │
└────────────────────────────────────────────────────────────────────────────┘
```

#### Language Selection Overlay (`Ctrl+L`)
Quickly select one or more languages for combined filtering.
Use `Space` to toggle selection, `Enter` to confirm, and `Esc` to cancel.

```
┌──────────── Select Languages ────────────────┐
│ Space: toggle, Enter: confirm, Esc: cancel   │
│ →  [x] Go                                    │
│    [ ] Python                                │
│    [x] Markdown                              │
│ ...                                          │
└──────────────────────────────────────────────┘
```

#### Language Distribution Chart (`Ctrl+w`)
Visually represents the distribution of programming languages in the project.
```
┌───────────────────────────────────────────────┐
│                  Language Distribution        │
│    ███████                                    │
│  ███████████     █ Go: 18,500 lines           │
│  ███████████     █ YAML: 2,300 lines          │
│    ███████       █ Markdown: 450 lines        │
│      ███         ...                          │
└───────────────────────────────────────────────┘
```

#### Tree Mode (`t`)
Toggle tree mode to expand and collapse directories inline without navigating into them. Supports nested expansion.
```
┌─ 📊 Code Statistics ───────────────────────────────────────────────────────┐
│  ICON  NAME                    LANGUAGES        CODE      TOTAL         %  │
│  📂    src                     Go, ...          15,021    18,345    65.3%  │
│  ▾     internal                Go, ...          8,200     10,100    35.7%  │
│          📂 api                Go               4,000     5,000     17.6%  │
│          💻 handler.go         Go               1,200     1,500      5.3%  │
│          💻 service.go         Go               800       1,000      3.5%  │
│  📄    main.go                 Go               850       1,010     3.6%   │
│  ...                                                                       │
└────────────────────────────────────────────────────────────────────────────┘
```

#### File Preview (`Enter` on files)
Instantly preview file contents in a scrollable overlay window.
```
┌──────────────────── File Preview: main.go ─────────────────────────┐
│                                                                    │
│  package main                                                      │
│                                                                    │
│  import (                                                          │
│      "fmt"                                                         │
│      "os"                                                          │
│  )                                                                 │
│                                                                    │
│  func main() {                                                     │
│      fmt.Println("Hello, World!")                                  │
│  }                                                                 │
│                                                                    │
│  Press 'q' to close, ↑/↓/j/k to scroll, PgUp/PgDn for page   15/23 │
└────────────────────────────────────────────────────────────────────┘
```

## ⚠️ Prerequisites

**Pre-built release binaries have tokei embedded**—no manual installation is needed.

If you are building from source or want to use your own tokei installation, ensure `tokei` is available in your `PATH`. Tokui will automatically prefer the system-installed version if present.

- **Install `tokei`** (optional): [https://github.com/XAMPPRocky/tokei#installation](https://github.com/XAMPPRocky/tokei#installation)

## 📦 Installation

#### Pre-compiled Binaries
You can download the latest pre-compiled version from the [Releases](https://github.com/zdyxry/tokui/releases) page. Unzip the file and it's ready to use—no extra installation required.

#### Build from Source (Go 1.24+ required)
```bash
# Clone the repository
git clone https://github.com/zdyxry/tokui.git
cd tokui

# Download tokei binaries for embedding (optional but recommended)
make fetch-tokei-binaries

# Build
make build

# Run
./bin/tokui
```

> **Note**: `make fetch-tokei-binaries` downloads platform-specific tokei binaries that will be embedded into the final executable. If skipped, the build will still succeed using placeholder files; the resulting binary will fallback to a system-installed tokei at runtime.

## 🛠️ Usage

Tokui supports two modes of operation:

#### 1. Direct Mode (Recommended)
Tokui will automatically invoke the bundled (or system-installed) `tokei` to analyze the specified directory.

```bash
# Analyze the current directory
tokui

# Analyze a specific directory
tokui /path/to/your/project
```

#### 2. Pipe Mode
If you have `tokei` installed separately, you can run it manually with custom arguments and pipe its JSON output to `tokui`. This is useful for advanced filtering (e.g., `--exclude`).

```bash
# Analyze the current directory
tokei -o json . | tokui

# Analyze a specific directory and exclude node_modules
tokei -o json --exclude node_modules . | tokui
```

### CLI Arguments

Tokui currently supports the following command-line flags:

```
Usage:
  tokui [directory] [flags]

Flags:
  -r, --root string   Specify the root directory to analyze. Defaults to the current directory ".".
                      Example: tokui --root="/path/to/project"

  -t, --tree          Start in tree mode. Directories are expandable inline instead of navigable.
                      Example: tokui --tree /path/to/project

  -h, --help          Show help information
```

## ⌨️ Keybindings

| Key                 | Action                                                              |
| ------------------- | ------------------------------------------------------------------- |
| `↑` / `k`           | Move cursor up                                                      |
| `↓` / `j`           | Move cursor down                                                    |
| `Enter`             | Enter directory (Nav mode) / Expand-Collapse directory (Tree mode) / Preview file |
| `e`                 | Open file in editor                                                 |
| `Backspace`         | Go back to the parent directory                                     |
| `t`                 | Toggle between navigation mode and tree mode                        |
| `Tab`               | Cycle through language filters (All, Go, Python, ...)               |
| `Ctrl`+`L`          | Open multi-language selection overlay (multi-select language filter) |
| `/`                 | Activate/input file name filter (press `Esc` to exit filter mode)   |
| `Ctrl`+`w`          | Show/hide language distribution pie chart                           |
| `?`                 | Show/hide full help                                                 |
| `q` / `Ctrl`+`c`    | Quit the application / Close file preview                           |

### File Preview Mode

When previewing a file, additional keyboard shortcuts are available:

| Key                 | Action                                                              |
| ------------------- | ------------------------------------------------------------------- |
| `↑` / `k`           | Scroll up                                                           |
| `↓` / `j`           | Scroll down                                                         |
| `PgUp`              | Page up                                                             |
| `PgDn`              | Page down                                                           |
| `q` / `Esc`         | Close file preview and return to directory view                     |

## 🤝 Contributing

Pull Requests are welcome! If you'd like to add new features or report bugs, please open an issue first to discuss your ideas.

## 📝 License

Tokui is licensed under the [MIT License](./LICENSE).

This project bundles a copy of [tokei](https://github.com/XAMPPRocky/tokei) (MIT OR Apache-2.0) for zero-dependency operation. See [THIRD-PARTY-LICENSES](./THIRD-PARTY-LICENSES) for details.
