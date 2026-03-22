# Articles 分页和过滤功能设计

## 概述

为 `articles` 命令添加分页和阅读状态过滤功能。

## CLI Flags

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--page` | int | 1 | 页码 |
| `--per-page` | int | 20 | 每页条数 |
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

```
Unread articles (page 1/15, 283 total):

  [1] [new] 文章标题
       Blog: xxx
       URL: https://...
```

- 第一行显示状态、页码信息和总数
- 状态：`Unread articles`、`Read articles`、`All articles`

## 代码改动

### 1. storage/database.go

修改 `ListArticles` 函数签名：

```go
func (db *Database) ListArticles(unreadOnly *bool, blogID *int64, page int, perPage int) ([]model.Article, error)
```

- `unreadOnly`: `nil` = 全部，`true` = 未读，`false` = 已读
- `perPage = 0`：返回所有记录（不分页），用于内部调用
- SQL: `SELECT ... FROM articles WHERE ... ORDER BY discovered_date DESC LIMIT ? OFFSET ?`

新增 `CountArticles` 函数：

```go
func (db *Database) CountArticles(unreadOnly *bool, blogID *int64) (int, error)
```

- 返回符合条件的文章总数

### 2. storage/database_test.go

更新现有测试用例以适配新签名。

### 3. controller/controller.go

修改 `GetArticles` 函数：

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

**保持 `MarkAllArticlesRead` 不变**：调用 `ListArticles` 时使用 `perPage = 0` 获取所有未读文章。

### 4. controller/controller_test.go

更新现有测试用例以适配新签名。

### 5. cli/commands.go

修改 `newArticlesCommand`：

- 添加 `--page`、`--per-page`、`--read` flags
- 更新输出逻辑显示分页信息

**保持 `newReadAllCommand` 不变**：继续使用 `GetArticles` 或直接调用数据库获取未读文章列表进行标记。

### 6. scanner/scanner_test.go

更新 `scanner_test.go:52` 处对 `ListArticles` 的调用。

## 影响范围

以下位置调用 `ListArticles`，需更新调用方式：

| 文件 | 行号 | 用途 | 处理方式 |
|------|------|------|----------|
| `controller.go:81` | GetArticles | 分页查询 | 传入 page/perPage |
| `controller.go:127` | MarkAllArticlesRead | 获取全部未读 | perPage = 0 |
| `scanner_test.go:52` | 测试 | 测试用途 | perPage = 0 |

## 测试计划

### 单元测试

- `ListArticles`：
  - 未读/已读/全部三种状态
  - 分页参数：page=1, page=2, perPage=10
  - perPage=0 返回全部
  - 无效输入：page=0, page=-1, perPage=-1

- `CountArticles`：
  - 未读/已读/全部三种状态
  - 结合 blogID 过滤

### 集成测试

- CLI 命令各种 flag 组合：
  - `articles --read`
  - `articles --all --page 2`
  - `articles --blog "xxx" --read --page 1 --per-page 5`

### 边界情况

- 空结果
- 页码超出范围
- 单页结果（总数 <= perPage）