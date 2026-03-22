package controller

import (
	"fmt"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

type BlogNotFoundError struct {
	Name string
}

func (e BlogNotFoundError) Error() string {
	return fmt.Sprintf("Blog '%s' not found", e.Name)
}

type BlogAlreadyExistsError struct {
	Field string
	Value string
}

func (e BlogAlreadyExistsError) Error() string {
	return fmt.Sprintf("Blog with %s '%s' already exists", e.Field, e.Value)
}

type ArticleNotFoundError struct {
	ID int64
}

func (e ArticleNotFoundError) Error() string {
	return fmt.Sprintf("Article %d not found", e.ID)
}

func AddBlog(db *storage.Database, name string, url string, feedURL string, scrapeSelector string) (model.Blog, error) {
	if existing, err := db.GetBlogByName(name); err != nil {
		return model.Blog{}, err
	} else if existing != nil {
		return model.Blog{}, BlogAlreadyExistsError{Field: "name", Value: name}
	}
	if existing, err := db.GetBlogByURL(url); err != nil {
		return model.Blog{}, err
	} else if existing != nil {
		return model.Blog{}, BlogAlreadyExistsError{Field: "URL", Value: url}
	}

	blog := model.Blog{
		Name:           name,
		URL:            url,
		FeedURL:        feedURL,
		ScrapeSelector: scrapeSelector,
	}
	return db.AddBlog(blog)
}

func RemoveBlog(db *storage.Database, name string) error {
	blog, err := db.GetBlogByName(name)
	if err != nil {
		return err
	}
	if blog == nil {
		return BlogNotFoundError{Name: name}
	}
	_, err = db.RemoveBlog(blog.ID)
	return err
}

// ArticlesResult contains paginated articles and metadata.
type ArticlesResult struct {
	Articles   []model.Article
	BlogNames  map[int64]string
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// GetArticles retrieves paginated articles with filtering.
// status: "unread", "read", or "all"
func GetArticles(db *storage.Database, status string, blogName string, page int, perPage int) (*ArticlesResult, error) {
	var blogID *int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		blogID = &blog.ID
	}

	// Determine read status filter
	var readFilter *bool
	switch status {
	case "unread":
		unread := true
		readFilter = &unread
	case "read":
		read := false
		readFilter = &read
	case "all":
		readFilter = nil
	default:
		unread := true
		readFilter = &unread
	}

	// Get total count
	total, err := db.CountArticles(readFilter, blogID)
	if err != nil {
		return nil, err
	}

	// Get paginated articles
	articles, err := db.ListArticles(readFilter, blogID, page, perPage)
	if err != nil {
		return nil, err
	}

	// Get blog names
	blogs, err := db.ListBlogs()
	if err != nil {
		return nil, err
	}
	blogNames := make(map[int64]string)
	for _, blog := range blogs {
		blogNames[blog.ID] = blog.Name
	}

	// Calculate total pages
	totalPages := 0
	if perPage > 0 && total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	return &ArticlesResult{
		Articles:   articles,
		BlogNames:  blogNames,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

func MarkArticleRead(db *storage.Database, articleID int64) (model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return model.Article{}, err
	}
	if article == nil {
		return model.Article{}, ArticleNotFoundError{ID: articleID}
	}
	if !article.IsRead {
		_, err = db.MarkArticleRead(articleID)
		if err != nil {
			return model.Article{}, err
		}
	}
	return *article, nil
}

func MarkAllArticlesRead(db *storage.Database, blogName string) ([]model.Article, error) {
	var blogID *int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		blogID = &blog.ID
	}

	unread := true
	articles, err := db.ListArticles(&unread, blogID, 1, storage.NoPagination)
	if err != nil {
		return nil, err
	}

	for _, article := range articles {
		_, err := db.MarkArticleRead(article.ID)
		if err != nil {
			return nil, err
		}
	}

	return articles, nil
}

func MarkArticleUnread(db *storage.Database, articleID int64) (model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return model.Article{}, err
	}
	if article == nil {
		return model.Article{}, ArticleNotFoundError{ID: articleID}
	}
	if article.IsRead {
		_, err = db.MarkArticleUnread(articleID)
		if err != nil {
			return model.Article{}, err
		}
	}
	return *article, nil
}
