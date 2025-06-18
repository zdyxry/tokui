# 📊 Tokui

[![Build](https://github.com/zdyxry/tokui/actions/workflows/build.yml/badge.svg)](https://github.com/zdyxry/tokui/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/zdyxry/tokui)](https://goreportcard.com/report/github.com/zdyxry/tokui)

**Tokui** is a high-performance, cross-platform command-line tool for visualizing and exploring your code statistics. It integrates with the powerful code statistics engine [tokei](https://github.com/XAMPPRocky/tokei) to present code line count metrics through a responsive, keyboard-driven Terminal User Interface (TUI), helping you to quickly analyze code composition and understand your project's structure.

> **Project Origin**
>
> This project is a fork of the excellent disk space analyzer [noxdir](https://github.com/crumbyte/noxdir), and was heavily modified and refactored by a **Large Language Model (LLM)**, transforming its functionality from a disk analyzer into a code statistics visualizer.

## ✨ Features

- **Interactive Terminal UI**: Navigate through your project's directory structure with ease using your keyboard.
- **Deep Tokei Integration**: Leverages the powerful analysis capabilities of `tokei` for accurate code statistics.
- **Detailed Data Analysis**: Displays lines of code, comments, blanks, and total lines, categorized by language.
- **File Preview**: Press `Enter` on any file to instantly preview its contents in a scrollable overlay window.
- **Visual Charts**: Toggle a pie chart with `Ctrl+w` to intuitively visualize the proportion of each language.
- **Dynamic Filtering and Searching**:
  - Quickly search by file name (`/`).
  - Filter statistics by a specific language (`Tab`).
- **High-Performance & Lightweight**: Written in Go, it compiles to a single, installation-free binary.
- **Privacy-Focused**: Runs entirely locally. No telemetry or data uploads, ever.

## 📸 Previews

#### Main View
A clean table showing code statistics for files and subdirectories in the current path.
```
┌─ 📊 Code Statistics ───────────────────────────────────────────────────────┐
│  ICON  NAME                    LANGUAGES        CODE      TOTAL         %   │
│  📂    internal                Go, ...          15,021    18,345    65.3%  │
│  💻    main.go                 Go               850       1,010     3.6%   │
│  📜    README.md               Markdown         120       150       0.5%   │
│  ...                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Language Distribution Chart (`Ctrl+w`)
Visually represents the distribution of programming languages in the project.
```
┌───────────────────────────────────────────────┐
│                  Language Distribution        │
│    ███████                                    │
│  ███████████     █ Go: 18,500 lines            │
│  ███████████     █ YAML: 2,300 lines           │
│    ███████       █ Markdown: 450 lines         │
│      ███         ...                           │
└───────────────────────────────────────────────┘
```

#### File Preview (`Enter` on files)
Instantly preview file contents in a scrollable overlay window.
```
┌──────────────────── File Preview: main.go ────────────────────────┐
│                                                                    │
│  package main                                                      │
│                                                                    │
│  import (                                                          │
│      "fmt"                                                         │
│      "os"                                                          │
│  )                                                                 │
│                                                                    │
│  func main() {                                                     │
│      fmt.Println("Hello, World!")                                 │
│  }                                                                 │
│                                                                    │
│  Press 'q' to close, ↑/↓/j/k to scroll, PgUp/PgDn for page   15/23 │
└────────────────────────────────────────────────────────────────────┘
```

## ⚠️ Prerequisites

**Tokui has a strong dependency on `tokei`**. Before using, please ensure you have `tokei` installed and added to your system's `PATH` environment variable.

- **Install `tokei`**: [https://github.com/XAMPPRocky/tokei#installation](https://github.com/XAMPPRocky/tokei#installation)

## 📦 Installation

#### Pre-compiled Binaries
You can download the latest pre-compiled version from the [Releases](https://github.com/zdyxry/tokui/releases) page. Unzip the file and it's ready to use—no extra installation required.

#### Build from Source (Go 1.24+ required)
```bash
# Clone the repository
git clone https://github.com/zdyxry/tokui.git
cd tokui

# Build
make build

# Run
./bin/tokui
```

## 🛠️ Usage

Tokui supports two modes of operation:

#### 1. Direct Mode (Recommended)
In this mode, Tokui will automatically call the `tokei` command to analyze the specified directory.

```bash
# Analyze the current directory
tokui

# Analyze a specific directory
tokui /path/to/your/project
```

#### 2. Pipe Mode
You can first run `tokei` manually and pipe its `json` output to `tokui`. This is useful when you need to use more complex arguments for `tokei`.

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

  -h, --help          Show help information
```

## ⌨️ Keybindings

| Key                 | Action                                                              |
| ------------------- | ------------------------------------------------------------------- |
| `↑` / `k`           | Move cursor up                                                      |
| `↓` / `j`           | Move cursor down                                                    |
| `Enter`             | Enter selected directory / Preview file content                    |
| `Backspace`         | Go back to the parent directory                                     |
| `Tab`               | Cycle through language filters (All, Go, Python, ...)               |
| `/`                 | Activate/input file name filter (press `Esc` to exit filter mode)   |
| `Ctrl`+`w`          | Show/hide language distribution pie chart                         |
| `?`                 | Show/hide full help                                                 |
| `q` / `Ctrl`+`c`    | Quit the application / Close file preview                          |

### File Preview Mode

When previewing a file, additional keyboard shortcuts are available:

| Key                 | Action                                                              |
| ------------------- | ------------------------------------------------------------------- |
| `↑` / `k`           | Scroll up                                                           |
| `↓` / `j`           | Scroll down                                                         |
| `PgUp`              | Page up                                                             |
| `PgDn`              | Page down                                                           |
| `q` / `Esc`         | Close file preview and return to directory view                    |

## 🤝 Contributing

Pull Requests are welcome! If you'd like to add new features or report bugs, please open an issue first to discuss your ideas.

## 📝 License

MIT © [crumbyte](https://github.com/crumbyte), [zdyxry](https://github.com/zdyxry)