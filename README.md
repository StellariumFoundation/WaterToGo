# WaterToGo

Convert JS/TS/Python/Rust codebases to idiomatic Go using Google Gemini.

![logo](assets/logo.png)

## Download

Grab the latest release for your platform from the [Releases](https://github.com/johnvictor/watertogo/releases) page.

| Platform | File | How to run |
|---|---|---|
| **Windows** (x86_64) | `WaterToGo-x86_64.exe` | Double-click |
| **Windows** (ARM64) | `WaterToGo-arm64.exe` | Double-click |
| **Linux** (x86_64) | `WaterToGo-x86_64.AppImage` | Double-click or `chmod +x && ./` |
| **Linux** (ARM64) | `WaterToGo-arm64.AppImage` | Double-click or `chmod +x && ./` |
| **macOS** (Intel) | `WaterToGo-darwin-x86_64.tar.gz` | Extract, double-click `WaterToGo.app` |
| **macOS** (Apple Silicon) | `WaterToGo-darwin-arm64.tar.gz` | Extract, double-click `WaterToGo.app` |

**Windows**: The `.exe` has your logo embedded — it shows in File Explorer. Double-click to open a terminal and launch the TUI.

**Linux**: The AppImage bundles the binary, icon, and `.desktop` entry in one file. Double-click to run in a terminal.

**macOS**: Extract the `.tar.gz`, then double-click `WaterToGo.app` in Finder — it opens Terminal automatically.

## Usage

1. **Get a Gemini API key** at [Google AI Studio](https://aistudio.google.com/apikey).
2. Run WaterToGo. Paste your API key(s) on the first screen.
3. Enter the path to the project folder you want to convert.
4. Watch it convert — all output goes into a `0go0/` subfolder alongside your project.

You can enter multiple API keys separated by commas. If one key hits a quota limit, WaterToGo automatically rotates to the next key and falls back through 12 different models.

## Build from source

```bash
git clone https://github.com/johnvictor/watertogo.git
cd watertogo
go build -o watertogo .
```

Requires Go 1.26+.

## How it works

WaterToGo scans a project directory, identifies JS/TS/Python/Rust files, and sends each one to a Gemini chat session with a prompt asking for an idiomatic Go translation. Large files are split into chunks while keeping the full source in context so the model understands the complete structure. If a request fails, the tool immediately rotates to the next API key and falls back through 12 models.

The scanner respects `.gitignore` rules and skips `node_modules`, `.git`, build outputs, and `watertogo_config.json`.

## License

MIT
