# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MediaNinja is a Go-based media scraping tool that downloads content from various websites. It uses a modular, plugin-based parser architecture with concurrent download capabilities.

**Key Technologies**: Go 1.23, Cobra CLI, goquery (HTML parsing), m3u8 (streaming), logrus (logging)

## Common Development Commands

### Building and Running
```bash
# Build the application
go build -o MediaNinja

# Run with basic parameters
./MediaNinja --url https://example.com/media-page

# Run with proxy and custom settings
./MediaNinja --url https://example.com/media-page --proxy http://127.0.0.1:7890 --concurrency 10 --output ./downloads

# Install dependencies
go mod download

# Run tests
go test ./...
```

### Prerequisites
- Go 1.23+
- ffmpeg (for media processing)

## Architecture Overview

### Entry Point and Bootstrap
- `main.go`: Simple entry point that calls `cmd.Execute()`
- `cmd/root.go`: Cobra CLI implementation with PreRun hook for output directory setup
- Bootstrap process: Creates config → Sets up output directory → Instantiates crawler → Starts crawling

### Core Components

**Crawler Engine** (`core/crawler/crawler.go`)
- Main orchestrator with composition-based design
- Coordinates parser selection, HTML fetching, content parsing, and concurrent downloads
- Uses `concurrent.Limiter` for controlled parallelism
- Manages file organization with title-based directory structure

**Configuration** (`core/config/config.go`)
- Simple struct-based configuration with factory pattern
- Global config instance created at startup
- Key fields: URL, ProxyURL, Concurrency, OutputDir, MaxRetries, RetryDelay

**Parser System** (`core/parsers/`)
- Interface-based architecture with `Parser` and `Downloader` interfaces
- Factory pattern for parser selection based on URL string matching
- `ParseResult` struct with MediaInfo, FileContent, and metadata
- Site-specific parsers: Telegraph, DDYS, NTDM, Pornhub

**HTTP Client** (`core/request/client/`)
- Dual transport support (HTTP/1.1 and HTTP/2)
- Proxy support and configurable retry mechanisms
- Header management with default and request-specific headers

### Key Design Patterns

**Parser Interface Pattern**
```go
type Parser interface {
    Parse(html string) (*ParseResult, error)
    GetDownloader() Downloader
}
```

**Concurrent Processing**
- Channel-based concurrency limiter with WaitGroup synchronization
- Worker pattern for parallel media downloads
- Result collection through channels

**File Organization**
- Structured output: `{baseDir}/{titleDir}/{mediaType}/{filename}`
- Content type separation (images, videos, subtitles, files)
- Path sanitization for cross-platform compatibility

**Error Handling**
- Error wrapping with context: `fmt.Errorf("failed to %w: %v", ...)`
- Graceful degradation - individual failures don't stop entire crawl
- Structured logging with logrus

## Adding New Website Support

1. **Create Parser Struct**: Embed `DefaultDownloader` or implement custom downloader
2. **Implement Parser Interface**: Add `Parse()` method using goquery for HTML parsing
3. **Register in Factory**: Update `GetParser()` function with URL pattern matching
4. **Handle Media Types**: Return `MediaInfo` with appropriate MediaType (Image, Video, Subtitle)
5. **Test**: Add tests following pattern in `client_test.go`

**Example Parser Structure**:
```go
type MySiteParser struct {
    DefaultDownloader
}

func (p *MySiteParser) Parse(html string) (*ParseResult, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }

    result := &ParseResult{}
    // Extract media, title, content using goquery
    return result, nil
}
```

## Important Implementation Details

**URL Handling**
- All parsers should normalize relative URLs to absolute URLs
- Use `url.Parse()` for proper URL handling
- Handle URL prefixes in download methods when needed

**Content Security**
- The codebase handles media content responsibly for authorized use cases
- Parsers should only extract publicly accessible media content
- Implement proper error handling for restricted content

**Concurrent Downloads**
- Use the provided `concurrent.Limiter` for rate limiting
- Default concurrency is 5, configurable via CLI parameter
- Each media download runs in a separate goroutine with controlled parallelism

**Testing Patterns**
- Tests use table-driven patterns with struct slices
- Focus on testing HTTP client functionality and proxy support
- Error cases tested alongside success cases

## File Structure Key Points

- `core/`: Main application logic (crawler, parsers, config, request)
- `utils/`: Utility modules (concurrent, io, logger, format)
- `cmd/`: CLI implementation
- Site-specific parsers in `core/parsers/` follow naming convention: `{site}.go`
- Output files organized by media type: images/, videos/, subtitles/, files/