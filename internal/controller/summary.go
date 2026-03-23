package controller

import (
	"context"

	"github.com/Hyaxia/blogwatcher/internal/html"
	"github.com/Hyaxia/blogwatcher/internal/llm"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

// GetArticleContent returns the best available content for summarization.
// Priority: Content > FeedSummary > Description
func GetArticleContent(article *model.Article) string {
	if article.Content != "" {
		return article.Content
	}
	if article.FeedSummary != "" {
		return article.FeedSummary
	}
	return article.Description
}

// GenerateSummary generates an LLM summary for a single article.
func GenerateSummary(ctx context.Context, db *storage.Database, client *llm.Client, articleID int64, force bool) (*model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, ArticleNotFoundError{ID: articleID}
	}

	if article.Summary != "" && !force {
		return article, nil
	}

	content := GetArticleContent(article)
	if content == "" {
		return nil, llm.NoContentError{ArticleID: articleID}
	}

	plainText := html.ToPlainText(content)
	if plainText == "" {
		return nil, llm.NoContentError{ArticleID: articleID}
	}

	summary, err := client.Summarize(ctx, article.Title, plainText)
	if err != nil {
		return nil, err
	}

	if err := db.UpdateArticleSummary(articleID, summary); err != nil {
		return nil, err
	}

	article.Summary = summary
	return article, nil
}

// SummaryResult represents the result of a single summary generation.
type SummaryResult struct {
	ArticleID int64
	Title     string
	Status    string // "generated", "skipped", "error"
	Error     error
}

// GenerateAllSummaries generates summaries for all articles without one.
func GenerateAllSummaries(ctx context.Context, db *storage.Database, client *llm.Client, force bool) []SummaryResult {
	articles, err := db.ListArticles(nil, nil, 1, storage.NoPagination)
	if err != nil {
		return []SummaryResult{{Status: "error", Error: err}}
	}

	var results []SummaryResult
	for _, article := range articles {
		if article.Summary != "" && !force {
			continue
		}

		result := SummaryResult{
			ArticleID: article.ID,
			Title:     article.Title,
		}

		_, err := GenerateSummary(ctx, db, client, article.ID, force)
		if err != nil {
			result.Status = "error"
			result.Error = err
		} else {
			result.Status = "generated"
		}

		results = append(results, result)
	}

	return results
}