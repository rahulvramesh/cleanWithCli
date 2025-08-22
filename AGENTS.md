# Agent Instructions for Mac Storage Cleaner

## Build/Lint/Test Commands

### Build Commands
- `make build` - Build the application binary
- `make release` - Build optimized release version
- `make cross-compile` - Build for multiple Darwin architectures
- `go build -o mac-cleaner main.go` - Direct Go build command

### Test Commands
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `make bench` - Run benchmarks
- `go test -v ./...` - Run tests with verbose output
- `go test -v -race -coverprofile=coverage.out ./...` - Run tests with race detection and coverage

### Lint and Format Commands
- `make lint` - Run golangci-lint (requires installation)
- `make fmt` - Format Go code with go fmt
- `make vet` - Run go vet for static analysis
- `make check` - Run all checks (fmt, vet, lint, test)
- `golangci-lint run` - Direct linter command

### Development Commands
- `make run` - Run the application directly
- `make dev` - Build and run in development mode
- `make clean` - Clean build artifacts
- `make install-tools` - Install development tools (golangci-lint)

### Dependency Commands
- `make deps` - Download dependencies
- `make vendor` - Create vendor directory
- `make mod-tidy` - Tidy go.mod file
- `make deps-update` - Update all dependencies to latest versions
- `make deps-check` - Check for outdated dependencies

### Utility Commands
- `make help` - Show all available commands
- `make info` - Show project information
- `make size` - Show binary size
- `make cross-compile` - Build for multiple architectures

## Code Style Guidelines

### Go Conventions
- Follow standard Go formatting (`go fmt`)
- Use `gofmt` compatible formatting
- Run `go vet` for static analysis
- Use `golangci-lint` for comprehensive linting
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)

### Imports
- Group imports: standard library, blank line, third-party packages, blank line, local packages
- Use blank imports only when required for side effects
- Remove unused imports
- Use module path imports (e.g., `github.com/charmbracelet/bubbletea`)

### Error Handling
- Always handle errors appropriately
- Use `err != nil` pattern consistently
- Return errors from functions that can fail
- Use custom error types when appropriate
- Log errors with context when needed

### Types and Structs
- Use meaningful type names
- Define types close to where they're used
- Use struct embedding appropriately
- Follow Go's zero value philosophy

### Functions and Methods
- Keep functions focused and single-purpose
- Use descriptive function names
- Return errors as last return value
- Use pointer receivers for methods that modify state
- Use value receivers for small structs or when immutability is preferred

### Concurrency
- Use goroutines and channels appropriately
- Use sync.WaitGroup for coordinating goroutines
- Use sync.Mutex for protecting shared state
- Use atomic operations when appropriate
- Avoid race conditions

### Code Organization
- Group related functionality together
- Use clear package structure
- Keep main.go focused on application entry point
- Separate concerns (UI, business logic, file operations)

### Dependencies
- Use go modules for dependency management
- Keep dependencies minimal and up-to-date
- Vendor dependencies when deploying
- Use specific version tags, not branches

### Testing
- Write tests for all public functions
- Use table-driven tests when appropriate
- Test error conditions
- Use test coverage to identify untested code
- Follow Go testing conventions

### Documentation
- Document public APIs with comments
- Use package comments for package documentation
- Keep README and other docs up-to-date
- Use meaningful variable and function names to reduce need for comments

### Performance
- Profile code before optimizing
- Use efficient data structures
- Avoid unnecessary allocations
- Use buffered channels when appropriate
- Consider memory usage in long-running applications

### Security
- Validate user input
- Handle file permissions carefully
- Use secure defaults
- Avoid exposing sensitive information
- Follow principle of least privilege

### Bubble Tea TUI Guidelines
- Follow Bubble Tea patterns for model/view/update
- Use appropriate message types for state changes
- Handle window resizing gracefully
- Use lipgloss for consistent styling
- Implement proper key bindings

## Project-Specific Notes

### Architecture
- This is a terminal-based Mac storage cleaner
- Uses Bubble Tea for the TUI interface
- Follows MVC-like pattern with Model, Update, View functions
- Uses goroutines for parallel file scanning
- Implements proper error handling and user feedback

### Dependencies
- Uses charmbracelet ecosystem (bubbletea, bubbles, lipgloss)
- Uses go-humanize for file size formatting
- Minimal external dependencies to keep binary size small

### File Operations
- Always check file existence before operations
- Handle permission errors gracefully
- Use appropriate file walking functions
- Calculate directory sizes efficiently

### User Experience
- Provide clear progress feedback
- Use consistent color scheme and styling
- Handle edge cases (empty directories, permission issues)
- Provide helpful error messages

## Development Workflow

1. Run `make check` before committing to ensure code quality
2. Use `make test-coverage` to maintain test coverage
3. Run `make lint` to catch style and potential issues
4. Use `make dev` for development testing
5. Keep dependencies updated with `make deps-update`

## Commit Guidelines

- Use conventional commit format when possible
- Keep commits focused and atomic
- Test changes before committing
- Update documentation for significant changes
- Never commit secrets or sensitive information