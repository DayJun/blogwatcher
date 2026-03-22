# Articles 分页和过滤功能设计

## 概述

为 `articles` 命令添加分页和阅读状态过滤功能。

## CLI Flags

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--page` | int | 1 | 页码 |
| `--per-page` | int | 20 | 每页条数 |
| `--read` | bool | false | 只显示已读文章 |
| `--all` | bool | false | 显示全部（已读+未读） |
| `--blog` | string | "" | 按博客名称过滤（已有） |

### 行为规则

- 默认：显示未读文章，第 1 页，每页 20 条
- `--read`：只显示已读文章
- `--all`：显示全部文章（已读+未读）
- `--read` 与 `--all` 同时使用时，`--all` 优先

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
- 返回分页后的文章列表

新增 `CountArticles` 函数：

```go
func (db *Database) CountArticles(unreadOnly *bool, blogID *int64) (int, error)
```

- 返回符合条件的文章总数

### 2. controller/controller.go

修改 `GetArticles` 函数：

```go
type ArticlesResult struct {
    Articles  []model.Article
    BlogNames map[int64]string
    Total     int
    Page      int
    PerPage   int
    TotalPages int
}

func GetArticles(db *storage.Database, status string, blogName string, page int, perPage int) (*ArticlesResult, error)
```

- `status`: `"unread"`、`"read"` 或 `"all"`

### 3. cli/commands.go

修改 `newArticlesCommand`：

- 添加 `--page`、`--per-page`、`--read` flags
- 更新输出逻辑显示分页信息

## 测试计划

1. 单元测试：`ListArticles` 和 `CountArticles` 的各种参数组合
2. 集成测试：CLI 命令的各种 flag 组合
3. 边界情况：空结果、超出范围的页码