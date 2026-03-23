# CLI Commands Redesign

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign CLI commands for consistency, clarity, and LLM-friendliness.

**Architecture:** Reorganize commands into resource-based subcommands (blogs, articles) and action commands (init, scan, import, summary). All commands follow a consistent pattern with clear, unambiguous help text optimized for LLM understanding.

**Tech Stack:** Go 1.24, Cobra CLI framework

---

## Overview

### Current Problems

1. Inconsistent command structure (`add`, `remove` at top level vs `blogs`, `articles`)
2. Missing functionality (edit blog, show article/blog details)
3. `add` requires name even when feed URL can provide it
4. `summary --all` output is too verbose
5. `articles` cannot customize displayed fields
6. Help text not optimized for LLM consumption

### New Command Structure

```
init                     # Initialize configuration
scan                     # Scan for new articles
import <file>            # Import from OPML
summary [id]             # Generate LLM summary

blogs                    # List blogs
blogs <name>             # Show blog details
blogs add <url>          # Add blog (auto-name)
blogs add <name> <url>   # Add blog (custom name)
blogs edit <name>        # Edit blog
blogs remove <name>      # Remove blog

articles                 # List articles
articles <id>            # Show article details
articles read <id>       # Mark as read
articles unread <id>     # Mark as unread
articles read-all        # Mark all as read
```

---

## Pre-requisite State

All commands except `init` require configuration to be initialized.

**When not initialized (config.yaml missing or empty):**
```
Error: Not configured. Run 'blogwatcher init' first.
```

**When database doesn't exist:**
- Automatically created on first use
- No error needed

---

## Error Handling

### Common Errors (all commands)

| Error | Message | Exit Code |
|-------|---------|-----------|
| Not configured | `Error: Not configured. Run 'blogwatcher init' first.` | 1 |
| Network timeout | `Error: Request timeout after 30s` | 1 |
| Invalid URL format | `Error: Invalid URL: <url>` | 1 |

### blogs Errors

| Command | Condition | Message |
|---------|-----------|---------|
| `blogs <name>` | Blog not found | `Error: Blog '<name>' not found` |
| `blogs add` | Auto-name failed (no feed title) | `Error: Could not extract name from feed. Please provide a name.` |
| `blogs add` | Duplicate name | `Error: Blog with name '<name>' already exists` |
| `blogs add` | Duplicate URL | `Error: Blog with URL '<url>' already exists` |
| `blogs add` | Feed fetch failed | `Error: Failed to fetch feed: <reason>` |
| `blogs add` | Invalid URL | `Error: Invalid URL: <url>` |
| `blogs edit` | Blog not found | `Error: Blog '<name>' not found` |
| `blogs edit` | Duplicate new name | `Error: Blog with name '<name>' already exists` |
| `blogs remove` | Blog not found | `Error: Blog '<name>' not found` |

### articles Errors

| Command | Condition | Message |
|---------|-----------|---------|
| `articles <id>` | Article not found | `Error: Article <id> not found` |
| `articles read <id>` | Article not found | `Error: Article <id> not found` |
| `articles read <id>` | Already read | `Article <id> is already marked as read.` (success, no error) |
| `articles unread <id>` | Already unread | `Article <id> is already marked as unread.` (success, no error) |
| `articles read-all` | No unread articles | `No unread articles to mark as read.` (success, no error) |
| `articles read-all --blog <name>` | Blog not found | `Error: Blog '<name>' not found` |
| `articles --blog <name>` | Blog not found | `Error: Blog '<name>' not found` |
| `articles --per-page 0` | Invalid value | Auto-corrected to 20 (default) |
| `articles --per-page -5` | Invalid value | Auto-corrected to 20 (default) |
| `articles --per-page 150` | Over max | Auto-corrected to 100 (max) |

### scan Errors

| Condition | Message |
|-----------|---------|
| No blogs tracked | `No blogs tracked yet. Use 'blogwatcher blogs add' to add one.` |
| Blog not found | `Error: Blog '<name>' not found` |
| Feed parse failed | Shown in result per-blog, not as global error |
| Network failure | Shown in result per-blog, not as global error |

### import Errors

| Condition | Message |
|-----------|---------|
| File not found | `Error: File not found: <path>` |
| Invalid OPML | `Error: Failed to parse OPML: <reason>` |
| Empty OPML | `No feeds found in OPML file.` (success, no error) |

### summary Errors

| Condition | Message |
|-----------|---------|
| Not configured (no API key) | `Error: Not configured. Run 'blogwatcher init' first.` |
| Article not found | `Error: Article <id> not found` |
| No content available | Skipped with warning, not an error |
| API rate limit | `✗ Article <id>: API error: rate limit exceeded` |
| API error | `✗ Article <id>: API error: <message>` |
| No articles to process | `All articles already have summaries.` (success, no error) |

---

## Command Details

### init

Initialize configuration interactively.

```
Usage:
  blogwatcher init

Prompts for:
  - API Base URL (default: https://api.openai.com/v1)
  - API Key (required)
  - Model (default: gpt-4o-mini)

Creates:
  - ~/.blogwatcher/config.yaml
  - ~/.blogwatcher/blogwatcher.db

Errors:
  - If already configured: "Configuration already exists at ~/.blogwatcher/config.yaml"
  - Empty API key: "API Key is required."
```

### scan

Scan blogs for new articles.

```
Usage:
  blogwatcher scan [blog_name] [flags]

Arguments:
  blog_name    Optional. Scan only this blog. If omitted, scan all blogs.

Flags:
  -s, --silent       Only output "scan done" when complete
  -w, --workers num  Concurrent workers when scanning all (default: 8, min: 1, max: 20)

Output (non-silent):
  Scanning 3 blog(s)...

    Blog Name
      Source: RSS | Found: 10 | New: 3

    Another Blog
      Error: Failed to fetch feed: connection timeout

  Found 3 new article(s) total!
```

### import

Import blogs from OPML file.

```
Usage:
  blogwatcher import <file.opml>

Arguments:
  file.opml    Required. Path to OPML file.

Output:
  Importing from feeds.opml...

  ✓ Imported: "Tech Blog"
  ⚠ Skipped "Duplicate": Blog with URL already exists
  ✗ Failed: "Bad Feed" - Invalid feed URL

  Import complete: 5 imported, 2 skipped, 1 failed
```

### summary

Generate LLM summaries for articles.

```
Usage:
  blogwatcher summary [id] [flags]
  blogwatcher summary --all [flags]

Arguments:
  id           Article ID. Required unless --all is specified.

Flags:
      --all         Generate for all articles without summary
  -d, --days num    Only process articles discovered within last N days
                    Must be used with --all. Ignored without --all.
                    Default: 0 (no filter)
  -f, --force       Regenerate even if summary exists

Output for single article:
  Title: Understanding Go Concurrency

  Summary: This article introduces Go's concurrency model...

Output for --all:
  Only shows failures and summary count, not success lines.

  ⚠ Article 42: No content available
  ✗ Article 43: API error: rate limit exceeded

  Complete: 15 generated, 2 skipped, 1 failed

  (If all succeed, no per-article lines shown)
```

---

### blogs

Manage tracked blogs.

```
Usage:
  blogwatcher blogs                    # List all blogs
  blogwatcher blogs <name>             # Show blog details
  blogwatcher blogs add <url>          # Add with auto-name
  blogwatcher blogs add <name> <url>   # Add with custom name
  blogwatcher blogs edit <name>        # Edit blog
  blogwatcher blogs remove <name>      # Remove blog

Add flags:
      --feed-url url         RSS/Atom feed URL (auto-discovered from <url> if not provided)
      --scrape-selector sel  CSS selector for HTML scraping fallback

Edit flags:
      --name name            New blog name
      --feed-url url         New feed URL (set to "" to clear)
      --scrape-selector sel  New scrape selector (set to "" to clear)

Remove flags:
  -y, --yes                  Skip confirmation prompt
```

**blogs add auto-naming logic:**
1. If `--feed-url` provided or discovered: fetch feed, extract `<title>` as blog name
2. If feed has no title or fetch fails: error, user must provide name as positional arg
3. If name provided as positional arg: use that (skip auto-extraction)

**blogs list output:**
```
Tracked blogs (3):

  Tech Blog
    URL: https://example.com
    Feed: https://example.com/rss.xml
    Last scanned: 2024-01-15 10:30

  Another Blog
    URL: https://another.com
    Feed: (auto-discovered)
    Last scanned: never
```

**blogs <name> output:**
```
Blog: Tech Blog
URL: https://example.com
Feed: https://example.com/rss.xml
Selector: (none)
Last scanned: 2024-01-15 10:30
Articles: 42 total, 5 unread
```

**blogs edit <name> output:**
```
Blog 'Tech Blog' updated.
```

**blogs remove <name> output:**
```
Remove blog 'Tech Blog' and all its articles? [y/N]: y
Removed blog 'Tech Blog'
```

---

### articles

List and manage articles.

```
Usage:
  blogwatcher articles [flags]          # List articles
  blogwatcher articles <id> [flags]     # Show article details
  blogwatcher articles read <id>        # Mark as read
  blogwatcher articles unread <id>      # Mark as unread
  blogwatcher articles read-all [flags] # Mark all as read

Filtering flags:
  -a, --all             Show all articles (including read)
  -r, --read            Show only read articles
  -b, --blog name       Filter by blog name

Output flags:
  -f, --fields list     Fields to display (comma-separated)
                        Available: id, title, url, blog, published, discovered,
                                   read, content, description, feed_summary, summary
                        Default (list): id, title, blog, read, url, published
                        Default (detail): id, title, url, blog, published, discovered,
                                         read, content

Pagination flags:
  -p, --page num        Page number (default: 1, min: 1)
  -P, --per-page num    Articles per page (default: 20, min: 1, max: 100)

Read-all flags:
  -b, --blog name       Only mark articles from this blog
  -y, --yes             Skip confirmation prompt
```

**articles list output format:**
```
Unread articles (page 1/3, 45 total):

  [1] [new] First Article Title
       Blog: Tech Blog
       URL: https://example.com/1
       Published: 2024-01-10

  [2] [read] Another Article
       Blog: Tech Blog
       URL: https://example.com/2
       Published: 2024-01-09
```

**articles list with custom fields output format:**
```
Unread articles (page 1/1, 2 total):

  [1] First Article Title | Tech Blog | https://example.com/1
  [2] Another Article | Tech Blog | https://example.com/2
```
Fields are displayed in a single line, pipe-separated, in the order specified.

**articles <id> output (default fields):**
```
ID: 42
Title: Understanding Go Concurrency
URL: https://example.com/go-concurrency
Blog: Tech Blog
Published: 2024-01-10
Discovered: 2024-01-15
Read: No
Content:
  [Full article content...]
```

**articles <id> output (custom fields):**
Same key-value format, only showing requested fields.

**articles read <id> output:**
```
Marked article 42 as read
```
Or if already read:
```
Article 42 is already marked as read.
```

**articles read-all output:**
```
Mark 45 unread article(s) as read? [y/N]: y
Marked 45 article(s) as read
```
Or with `--yes`:
```
Marked 45 article(s) as read
```
Or if no unread:
```
No unread articles to mark as read.
```

---

## Deprecated Commands

Old top-level commands will be deprecated but still work:

| Old Command      | New Command              | Deprecation Message |
|------------------|--------------------------|---------------------|
| `add`            | `blogs add`              | "Warning: 'add' is deprecated, use 'blogs add'" |
| `remove`         | `blogs remove`           | "Warning: 'remove' is deprecated, use 'blogs remove'" |
| `read`           | `articles read`          | "Warning: 'read' is deprecated, use 'articles read'" |
| `unread`         | `articles unread`        | "Warning: 'unread' is deprecated, use 'articles unread'" |
| `read-all`       | `articles read-all`      | "Warning: 'read-all' is deprecated, use 'articles read-all'" |

Behavior:
- Command still executes
- Prints deprecation warning to stderr
- Exit code unchanged
- Hidden from `--help`

---

## Implementation Notes

### Cobra Structure

```
rootCmd
├── initCmd
├── scanCmd
├── importCmd
├── summaryCmd
├── blogsCmd (with subcommands)
│   ├── blogsListCmd (default, no subcommand)
│   ├── blogsShowCmd (positional arg name)
│   ├── blogsAddCmd
│   ├── blogsEditCmd
│   └── blogsRemoveCmd
└── articlesCmd (with subcommands)
    ├── articlesListCmd (default, no subcommand)
    ├── articlesShowCmd (positional arg id)
    ├── articlesReadCmd
    ├── articlesUnreadCmd
    └── articlesReadAllCmd
```

### Help Text Guidelines

1. **Short**: One line, imperative verb, describe what it does
2. **Usage**: Show all invocation patterns clearly
3. **Flags**: Group by purpose (filtering, output, pagination)
4. **Examples**: Common use cases with real values
5. **Defaults**: Always state default values

### Output Character Encoding

- Use emoji indicators (✓, ⚠, ✗) for status
- Graceful degradation: if terminal doesn't support Unicode, fall back to text:
  - ✓ → [OK]
  - ⚠ → [WARN]
  - ✗ → [ERR]