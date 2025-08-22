
# 🧹 Mac Storage Cleaner

A terminal-based utility for cleaning up storage space on macOS systems. This tool scans for and removes unnecessary files like cache files, logs, old downloads, and development artifacts to free up disk space.

## ✨ Features

- **Full System Scan**: Comprehensive scan of all cleanable file categories
- **Dev Scan**: Focused scan for development-related files (Xcode, Homebrew, Node.js)
- **Quick Clean**: Safe removal of temporary and cache files
- **Interactive TUI**: User-friendly terminal interface with Bubble Tea
- **Real-time Progress**: Live progress tracking during scans and cleaning
- **Disk Usage Report**: View detailed disk usage information
- **Parallel Processing**: Fast scanning using goroutines
- **Safe Operations**: Only removes files that are safe to delete

### File Categories Scanned

- **Cache Files**: System and application caches
- **Log Files**: System and application logs
- **Trash**: Files in the trash bin
- **Old Downloads**: Downloads older than 30 days
- **Xcode Files**: Derived data, archives, and simulator files
- **Homebrew Cache**: Homebrew package cache
- **Node Modules**: node_modules directories in projects

## 📋 Requirements

- **macOS** (Darwin-based systems)
- **Go 1.24.5** or later
- **Terminal** with color support (recommended)

## 🚀 Installation

### Option 1: Download Binary
```bash
# Download the latest release from GitHub
# Coming soon - releases will be available on the GitHub releases page
```

### Option 2: Build from Source
```bash
# Clone the repository
git clone https://github.com/rahulvramesh/cleanWithCli.git
cd cleanWithCli

# Build the application
make build
# or
go build -o mac-cleaner main.go

# Optional: Install globally
make install
```

### Option 3: Using Make
```bash
# Quick build and run
make dev

# Install development tools
make install-tools

# Run all checks
make check
```

## 🎯 Usage

### Basic Usage
```bash
# Run the application
./mac-cleaner

# Or if installed globally
mac-cleaner
```

### Navigation
- **↑/↓ or j/k**: Navigate menus
- **Enter**: Select option
- **Esc**: Go back
- **q**: Quit application

### Available Options
1. **Full System Scan**: Complete scan of all file categories
2. **Dev Scan**: Scan development-related files only
3. **Quick Clean**: Safe removal of temporary files
4. **Disk Usage Report**: View disk usage statistics
5. **Exit**: Quit the application

## 🛠️ Development

### Prerequisites
- Go 1.24.5+
- make (optional, for using Makefile commands)

### Setup
```bash
# Install dependencies
make deps

# Install development tools
make install-tools

# Run tests
make test

# Run with coverage
make test-coverage

# Run linter
make lint

# Run all checks
make check
```

### Available Make Commands
```bash
make help          # Show all available commands
make build         # Build the application
make run           # Run the application
make dev           # Build and run in development mode
make test          # Run tests
make test-coverage # Run tests with coverage
make lint          # Run linter
make fmt           # Format code
make vet           # Run go vet
make check         # Run all checks (fmt, vet, lint, test)
make clean         # Clean build artifacts
make install       # Install globally
make release       # Build optimized release
make cross-compile # Build for multiple architectures
```

### Project Structure
```
.
├── main.go           # Main application entry point
├── Makefile          # Build and development commands
├── go.mod           # Go module definition
├── go.sum           # Dependency checksums
├── README.md        # This file
└── AGENTS.md        # Agent instructions for AI assistants
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines
- Follow Go conventions and formatting
- Add tests for new features
- Update documentation as needed
- Run `make check` before committing
- Use conventional commit messages

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## ⚠️ Disclaimer

**Use this tool at your own risk.** While designed to be safe, always review what files will be deleted before proceeding. The authors are not responsible for any data loss.

## 🆘 Support

If you encounter issues:
1. Check the [Issues](https://github.com/rahulvramesh/cleanWithCli/issues) page
2. Create a new issue with detailed information
3. Include your macOS version and Go version

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [go-humanize](https://github.com/dustin/go-humanize) - File size formatting