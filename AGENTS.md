# Agent Instructions for Mac Storage Cleaner

## Build/Lint/Test Commands
- `make build` - Build the application binary
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `make lint` - Run golangci-lint (requires installation)
- `make fmt` - Format Go code with go fmt
- `make vet` - Run go vet for static analysis
- `make check` - Run all checks (fmt, vet, lint, test)
- `make run` - Run the application directly
- `make dev` - Build and run in development mode
- `make clean` - Clean build artifacts

## Code Style Guidelines

### Go Conventions
- Follow standard Go formatting (`go fmt`)
- Use `gofmt` compatible formatting
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)
- Use `err != nil` pattern consistently for error handling
- Return errors as last return value from functions

### Imports
- Group imports: standard library, blank line, third-party packages, blank line, local packages
- Remove unused imports
- Use module path imports (e.g., `github.com/charmbracelet/bubbletea`)

### Functions and Methods
- Keep functions focused and single-purpose
- Use descriptive function names
- Use pointer receivers for methods that modify state
- Use value receivers for small structs or when immutability is preferred

### Concurrency
- Use goroutines and channels appropriately
- Use sync.WaitGroup for coordinating goroutines
- Use sync.Mutex for protecting shared state
- Avoid race conditions

### Testing
- Write tests for all public functions
- Use table-driven tests when appropriate
- Test error conditions
- Follow Go testing conventions

### Bubble Tea TUI Guidelines
- Follow Bubble Tea patterns for model/view/update
- Use appropriate message types for state changes
- Handle window resizing gracefully
- Use lipgloss for consistent styling
- Implement proper key bindings

## Project-Specific Notes
- Terminal-based Mac storage cleaner using Bubble Tea TUI
- Uses charmbracelet ecosystem (bubbletea, bubbles, lipgloss)
- Uses go-humanize for file size formatting
- Implements parallel file scanning with goroutines
- Always check file existence before operations
- Handle permission errors gracefully

## Development Workflow
1. Run `make check` before committing to ensure code quality
2. Use `make test-coverage` to maintain test coverage
3. Run `make lint` to catch style and potential issues
4. Use `make dev` for development testing
5. Keep dependencies updated with `make deps-update`