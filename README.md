# BlogWatcher

A Go CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback. Includes LLM-powered article summarization.

## Features

-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **OPML Import** - Import blogs from OPML files exported from other RSS readers
-   **Read/Unread Management** - Track which articles you've read
-   **Blog Filtering** - View articles from specific blogs
-   **LLM Summarization** - Generate article summaries using OpenAI-compatible APIs
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Colored CLI Output** - User-friendly terminal interface

## Installation

```bash
# Homebrew (Linux)
brew install Hyaxia/tap/blogwatcher

# Install the CLI
go install github.com/Hyaxia/blogwatcher/cmd/blogwatcher@latest

# Or build locally
go build ./cmd/blogwatcher
```

Windows and Linux binaries are also available on the GitHub Releases page.

## Quick Start

```bash
# Initialize configuration (first time setup)
blogwatcher init

# Add a blog (auto-discovers RSS feed and extracts name from feed title)
blogwatcher blogs add https://example.com/blog

# Add a blog with a custom name
blogwatcher blogs add "My Favorite Blog" https://example.com/blog

# Scan for new articles
blogwatcher scan

# List unread articles
blogwatcher articles

# Generate summaries for today's articles
blogwatcher summary --all --days 1
```

## Configuration

Run `blogwatcher init` to set up LLM configuration interactively:

```bash
$ blogwatcher init

Welcome to BlogWatcher!

Let's set up your configuration.

API Base URL [https://api.openai.com/v1]: https://your-llm-api.com/v1
API Key: your-api-key
Model [gpt-4o-mini]: gpt-4o

✓ Configuration saved!
  Config: ~/.blogwatcher/config.yaml
  Database: ~/.blogwatcher/blogwatcher.db
```

Configuration is stored in `~/.blogwatcher/config.yaml`:

```yaml
llm:
  base_url: https://api.openai.com/v1
  api_key: your-api-key
  model: gpt-4o-mini
```

Any OpenAI-compatible API is supported (OpenAI, Azure OpenAI, vLLM, local models, etc.).

## Usage

### Adding Blogs

```bash
# Add a blog (auto-discovers RSS feed and extracts name from feed title)
blogwatcher blogs add https://example.com/blog

# Add a blog with a custom name
blogwatcher blogs add "My Favorite Blog" https://example.com/blog

# Add with explicit feed URL
blogwatcher blogs add "Tech Blog" https://techblog.com --feed-url https://techblog.com/rss.xml

# Add with HTML scraping selector (for blogs without feeds)
blogwatcher blogs add "No-RSS Blog" https://norss.com --scrape-selector "article h2 a"
```

### Importing Blogs

```bash
# Import blogs from an OPML file
blogwatcher import ~/Downloads/feeds.opml
```

OPML files exported from RSS readers (like Feedly, Inoreader, NetNewsWire) are supported. The import command:

-   Parses nested categories and extracts all feeds
-   Skips duplicates (blogs already in the database)
-   Shows a summary with imported, skipped, and failed counts

### Managing Blogs

```bash
# List all tracked blogs
blogwatcher blogs

# Show blog details with article stats
blogwatcher blogs "Tech Blog"

# Edit blog properties
blogwatcher blogs edit "Tech Blog" --name "New Name"
blogwatcher blogs edit "Tech Blog" --feed-url https://newfeed.com/rss.xml
blogwatcher blogs edit "Tech Blog" --scrape-selector "article a"

# Remove a blog (and all its articles)
blogwatcher blogs remove "My Favorite Blog"

# Remove without confirmation
blogwatcher blogs remove "My Favorite Blog" -y
```

### Scanning for New Articles

```bash
# Scan all blogs for new articles
blogwatcher scan

# Scan a specific blog
blogwatcher scan "Tech Blog"
```

### Viewing Articles

```bash
# List unread articles
blogwatcher articles

# Show article details
blogwatcher articles 42

# List all articles (including read)
blogwatcher articles --all

# List only read articles
blogwatcher articles --read

# List articles from a specific blog
blogwatcher articles --blog "Tech Blog"

# Customize displayed fields
blogwatcher articles --fields id,title,url
blogwatcher articles --fields id,title,blog,published

# Paginate results
blogwatcher articles --page 2 --per-page 50
```

### Managing Read Status

```bash
# Mark an article as read (use article ID from articles list)
blogwatcher articles read 42

# Mark an article as unread
blogwatcher articles unread 42

# Mark all unread articles as read
blogwatcher articles read-all

# Mark all unread articles as read for a blog (skip prompt)
blogwatcher articles read-all --blog "Tech Blog" --yes
```

### Generating Summaries

BlogWatcher can generate LLM-powered summaries for articles:

```bash
# Generate summary for a specific article
blogwatcher summary 42

# Generate summaries for all articles without one
blogwatcher summary --all

# Generate summaries for articles discovered today
blogwatcher summary --all --days 1

# Regenerate summaries (even if already exists)
blogwatcher summary --all --force
```

## How It Works

### Scanning Process

1. For each tracked blog, BlogWatcher first attempts to parse the RSS/Atom feed
2. If no feed URL is configured, it tries to auto-discover one from the blog homepage
3. If RSS parsing fails and a `scrape_selector` is configured, it falls back to HTML scraping
4. New articles are saved to the database as unread
5. Already-tracked articles are skipped

### Feed Auto-Discovery

BlogWatcher searches for feeds in two ways:

-   Looking for `<link rel="alternate">` tags with RSS/Atom types
-   Checking common feed paths: `/feed`, `/rss`, `/feed.xml`, `/atom.xml`, etc.

### HTML Scraping

When RSS isn't available, provide a CSS selector that matches article links:

```bash
# Example selectors
--scrape-selector "article h2 a"      # Links inside article h2 tags
--scrape-selector ".post-title a"     # Links with post-title class
--scrape-selector "#blog-posts a"     # Links inside blog-posts ID
```

## Data Storage

BlogWatcher stores data in `~/.blogwatcher/`:

-   **config.yaml** - LLM configuration (API key, base URL, model)
-   **blogwatcher.db** - SQLite database with:
    -   **blogs** - Tracked blogs (name, URL, feed URL, scrape selector)
    -   **articles** - Discovered articles (title, URL, dates, read status, summaries)

## Development

### Requirements

-   Go 1.24+

### Running Tests

```bash
# Run all tests
go test ./...
```

### Publishing

in addition to publishing to main a new tag should be published so homebrew will get the updated version:
```
  git tag vX.Y.Z
  git push origin vX.Y.Z
```

## License

MIT
