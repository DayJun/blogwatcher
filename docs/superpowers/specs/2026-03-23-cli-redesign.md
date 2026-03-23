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
init                     # 初始化配置
scan                     # 扫描新文章
import <file>            # 导入 OPML
summary [id]             # 生成摘要

blogs                    # 列出博客
blogs <name>             # 博客详情
blogs add <url>          # 添加博客（自动命名）
blogs add <name> <url>   # 添加博客（指定名称）
blogs edit <name>        # 编辑博客
blogs remove <name>      # 删除博客

articles                 # 列出文章
articles <id>            # 文章详情
articles read <id>       # 标记已读
articles unread <id>     # 标记未读
articles read-all        # 全部标记已读
```

---

## Command Details

### init (unchanged)

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
```

### scan (unchanged)

Scan blogs for new articles.

```
Usage:
  blogwatcher scan [blog_name] [flags]

Flags:
  -s, --silent       Only output "scan done" when complete
  -w, --workers num  Concurrent workers (default: 8)
```

### import (unchanged)

Import blogs from OPML file.

```
Usage:
  blogwatcher import <file.opml>
```

### summary

Generate LLM summaries for articles.

```
Usage:
  blogwatcher summary [id] [flags]

Flags:
      --all         Generate for all articles without summary
  -d, --days num    Only process articles discovered within last N days (with --all)
  -f, --force       Regenerate even if summary exists

Behavior changes:
  - summary <id>: Output "Title: ...\n\nSummary: ..."
  - summary --all: Only show failures/errors, then summary counts

Output for --all:
  ⚠ Article 42: No content available
  ✗ Article 43: API error: rate limit exceeded

  Complete: 15 generated, 2 skipped, 1 failed
```

---

### blogs

Manage tracked blogs.

```
Usage:
  blogwatcher blogs [flags]           # 列表
  blogwatcher blogs <name>            # 详情
  blogwatcher blogs add <url>         # 添加（自动命名）
  blogwatcher blogs add <name> <url>  # 添加（指定名称）
  blogwatcher blogs edit <name>       # 编辑
  blogwatcher blogs remove <name>     # 删除

Add flags:
      --feed-url url         RSS/Atom feed URL (auto-discovered if not provided)
      --scrape-selector sel  CSS selector for HTML scraping fallback

Edit flags:
      --name name            New name
      --feed-url url         New feed URL
      --scrape-selector sel  New scrape selector

Remove flags:
  -y, --yes                  Skip confirmation

Examples:
  blogwatcher blogs                                    # List all blogs
  blogwatcher blogs "Tech Blog"                        # Show details
  blogwatcher blogs add https://example.com            # Add with auto name
  blogwatcher blogs add "My Blog" https://example.com  # Add with custom name
  blogwatcher blogs add https://example.com --feed-url https://example.com/rss.xml
  blogwatcher blogs edit "Tech Blog" --name "Tech News"
  blogwatcher blogs remove "Old Blog" -y
```

**blogs add auto-naming logic:**
1. If `--feed-url` provided: fetch feed, extract `<title>` as blog name
2. If feed has no title or fetch fails: error, user must provide name
3. If name provided as positional arg: use that (no auto-extraction)

**blogs <name> output:**
```
Blog: Tech Blog
URL: https://example.com
Feed: https://example.com/rss.xml
Selector: (none)
Last scanned: 2024-01-15 10:30
Articles: 42 total, 5 unread
```

---

### articles

List and manage articles.

```
Usage:
  blogwatcher articles [flags]          # 列表
  blogwatcher articles <id> [flags]     # 详情
  blogwatcher articles read <id>        # 标记已读
  blogwatcher articles unread <id>      # 标记未读
  blogwatcher articles read-all [flags] # 全部已读

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
  -p, --page num        Page number (default: 1)
  -P, --per-page num    Articles per page, max 100 (default: 20)

Read-all flags:
  -b, --blog name       Only mark articles from this blog
  -y, --yes             Skip confirmation

Examples:
  blogwatcher articles                              # Unread articles, default fields
  blogwatcher articles -f id,title,summary          # Custom fields
  blogwatcher articles --all --blog "Tech Blog"     # All from specific blog
  blogwatcher articles 42                            # Show article details
  blogwatcher articles 42 -f title,content,summary  # Specific fields
  blogwatcher articles read 42                       # Mark as read
  blogwatcher articles read-all                      # Mark all as read
  blogwatcher articles read-all --blog "Tech Blog" -y
```

**articles <id> output:**
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

---

## Deprecated Commands

| Old Command      | New Command              |
|------------------|--------------------------|
| `add`            | `blogs add`              |
| `remove`         | `blogs remove`           |
| `read`           | `articles read`          |
| `unread`         | `articles unread`        |
| `read-all`       | `articles read-all`      |

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
│   ├── blogsListCmd (default)
│   ├── blogsShowCmd (positional arg)
│   ├── blogsAddCmd
│   ├── blogsEditCmd
│   └── blogsRemoveCmd
└── articlesCmd (with subcommands)
    ├── articlesListCmd (default)
    ├── articlesShowCmd (positional arg)
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

### Backward Compatibility

Old top-level commands (`add`, `remove`, `read`, `unread`, `read-all`) will be deprecated:
- Still work but print deprecation warning
- Hidden from help
- Removed in next major version