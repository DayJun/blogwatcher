# Articles 分页和过滤功能设计

## 概述

为 `articles` 命令添加分页和阅读状态过滤功能。

## CLI Flags

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--page` | int | 1 | 页码 |
| `--per-page` | int | 20 | 每页条数（最大 100） |
| `--read` | bool | false | 只显示已读文章（新增） |
| `--all` | bool | false | 显示全部（已读+未读，已有） |
| `--blog` | string | "" | 按博客名称过滤（已有） |

### 行为规则

- 默认：显示未读文章，第 1 页，每页 20 条
- `--read`：只显示已读文章（新增功能）
- `--all`：显示全部文章（已有行为，保持不变）
- `--read` 与 `--all` 同时使用时，`--all` 优先

### 输入验证

- `--page < 1`：自动修正为 1
- `--per-page < 1`：自动修正为 20
- `--per-page > 100`：自动修正为 100
- 页码超出总页数：返回空列表，显示 "page X/Y (no results)"

### 使用示例

```bash
articles                      # 未读，第1页，每页20条
articles --read               # 已读
articles --all                # 全部
articles --page 2             # 未读，第2页
articles --per-page 50        # 未读，每页50条
articles --read --page 3      # 已读，第3页
articles --blog "xxx" --read  # 某博客的已读文章
```

## 输出格式

从 `(count)` 格式改为 `(page X/Y, count total)` 格式：

```
Unread articles (page 1/15, 283 total):

  [1] [new] 文章标题
       Blog: xxx
       URL: https://...
```

- 第一行显示状态、页码信息和总数
- 状态：`Unread articles`、`Read articles`、`All articles`
- `total = 0` 时：`Unread articles (no results):`

## 代码改动

### 1. internal/storage/database.go

修改 `ListArticles` 函数签名：

```go
const NoPagination = 0  // 传递给 perPage 表示不分页

func (db *Database) ListArticles(unreadOnly *bool, blogID *int64, page int, perPage int) ([]model.Article, error)
```

- `unreadOnly`: `nil` = 全部，`true` = 未读，`false` = 已读
- `perPage = NoPagination`（0）：返回所有记录（不分页）
- SQL: `SELECT ... FROM articles WHERE ... ORDER BY discovered_date DESC LIMIT ? OFFSET ?`

新增 `CountArticles` 函数：

```go
func (db *Database) CountArticles(unreadOnly *bool, blogID *int64) (int, error)
```

### 2. internal/storage/database_test.go

更新现有测试用例以适配新签名。

### 3. internal/controller/controller.go

新增结果结构体和修改 `GetArticles` 函数：

```go
type ArticlesResult struct {
    Articles   []model.Article
    BlogNames  map[int64]string
    Total      int
    Page       int
    PerPage    int
    TotalPages int
}

func GetArticles(db *storage.Database, status string, blogName string, page int, perPage int) (*ArticlesResult, error)
```

- `status`: `"unread"`、`"read"` 或 `"all"`
- `TotalPages` 计算：`totalPages = (total + perPage - 1) / perPage`，`total = 0` 时为 0

**保持 `MarkAllArticlesRead` 不变**：调用 `ListArticles` 时使用 `perPage = storage.NoPagination` 获取所有未读文章。

### 4. internal/controller/controller_test.go

更新现有测试用例以适配新签名。

### 5. internal/cli/commands.go

修改 `newArticlesCommand`：

- 添加 `--page`、`--per-page`、`--read` flags
- 更新输出逻辑显示分页信息
- 输入验证：page/perPage 边界检查

**保持 `newReadAllCommand` 不变**：继续调用 `GetArticles` 获取未读文章列表进行标记。

### 6. internal/scanner/scanner_test.go

更新对 `ListArticles` 的调用，使用 `perPage = storage.NoPagination`。

## 影响范围

以下位置调用 `ListArticles`，需更新调用方式：

| 文件 | 调用位置 | 用途 | 处理方式 |
|------|----------|------|----------|
| `internal/controller/controller.go` | `GetArticles` 函数内 | 分页查询 | 传入 page/perPage |
| `internal/controller/controller.go` | `MarkAllArticlesRead` 函数内 | 获取全部未读 | `perPage = storage.NoPagination` |
| `internal/storage/database_test.go` | 多处测试 | 测试用途 | `perPage = storage.NoPagination` |
| `internal/scanner/scanner_test.go` | 测试代码 | 测试用途 | `perPage = storage.NoPagination` |

## 测试计划

### 单元测试

- `ListArticles`：
  - 未读/已读/全部三种状态
  - 分页参数：page=1, page=2, perPage=10
  - `perPage = NoPagination` 返回全部
  - 无效输入：page=0, page=-1, perPage=-1, perPage=200

- `CountArticles`：
  - 未读/已读/全部三种状态
  - 结合 blogID 过滤

### 集成测试

- CLI 命令各种 flag 组合：
  - `articles --read`
  - `articles --all --page 2`
  - `articles --blog "xxx" --read --page 1 --per-page 5`

### 边界情况

- 空结果（total = 0）
- 页码超出范围
- 单页结果（总数 <= perPage）
- `--per-page 100`（最大值）
- `--per-page 101`（自动修正为 100）