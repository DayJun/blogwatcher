package llm

import "fmt"

type MissingAPIKeyError struct{}

func (e MissingAPIKeyError) Error() string {
	return "OPENAI_API_KEY environment variable is not set"
}

type APICallError struct {
	StatusCode int
	Message    string
}

func (e APICallError) Error() string {
	return fmt.Sprintf("LLM API error (status %d): %s", e.StatusCode, e.Message)
}

type NoContentError struct {
	ArticleID int64
}

func (e NoContentError) Error() string {
	return fmt.Sprintf("Article %d has no content to summarize", e.ArticleID)
}