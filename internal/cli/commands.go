package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Hyaxia/blogwatcher/internal/config"
	"github.com/Hyaxia/blogwatcher/internal/controller"
	"github.com/Hyaxia/blogwatcher/internal/llm"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/opml"
	"github.com/Hyaxia/blogwatcher/internal/rss"
	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

func newBlogsCommand() *cobra.Command {
	var page int
	var perPage int
	var search string

	cmd := &cobra.Command{
		Use:   "blogs [id]",
		Short: "Manage tracked blogs.",
		Long: `Manage tracked blogs.

Without arguments, lists all tracked blogs.
With a blog ID argument, shows detailed information about that blog.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 0 {
				return runBlogsList(db, page, perPage, search)
			}
			return runBlogsShow(db, args[0])
		},
	}

	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Blogs per page (max 100)")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Filter by blog name (partial match)")

	cmd.AddCommand(newBlogsAddCommand())
	cmd.AddCommand(newBlogsEditCommand())
	cmd.AddCommand(newBlogsRemoveCommand())

	return cmd
}

func runBlogsList(db *storage.Database, page, perPage int, search string) error {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := controller.GetBlogsPaginated(db, page, perPage, search)
	if err != nil {
		printError(err)
		return markError(err)
	}

	if result.Total == 0 {
		if search != "" {
			color.New(color.FgCyan, color.Bold).Printf("No blogs found matching '%s'\n", search)
		} else {
			fmt.Println("No blogs tracked yet. Use 'blogwatcher blogs add' to add one.")
		}
		return nil
	}

	label := "Tracked blogs"
	if search != "" {
		label = fmt.Sprintf("Blogs matching '%s'", search)
	}
	color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)

	for _, blog := range result.Blogs {
		color.New(color.FgWhite, color.Bold).Printf("  [%d] %s\n", blog.ID, blog.Name)
		fmt.Printf("       URL: %s\n", blog.URL)
		if blog.FeedURL != "" {
			fmt.Printf("       Feed: %s\n", blog.FeedURL)
		} else {
			fmt.Println("       Feed: (auto-discovered)")
		}
		if blog.LastScanned != nil {
			fmt.Printf("       Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
		} else {
			fmt.Println("       Last scanned: never")
		}
		fmt.Println()
	}
	return nil
}

func runBlogsShow(db *storage.Database, idStr string) error {
	blogID, err := parseID(idStr)
	if err != nil {
		printError(fmt.Errorf("Invalid blog ID: %s", idStr))
		return markError(err)
	}

	blog, err := controller.GetBlogByID(db, blogID)
	if err != nil {
		printError(err)
		return markError(err)
	}

	stats, err := controller.GetBlogStats(db, blog.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Blog: %s\n", blog.Name)
	fmt.Printf("ID: %d\n", blog.ID)
	fmt.Printf("URL: %s\n", blog.URL)
	if blog.FeedURL != "" {
		fmt.Printf("Feed: %s\n", blog.FeedURL)
	} else {
		fmt.Println("Feed: (auto-discovered)")
	}
	if blog.ScrapeSelector != "" {
		fmt.Printf("Selector: %s\n", blog.ScrapeSelector)
	} else {
		fmt.Println("Selector: (none)")
	}
	if blog.LastScanned != nil {
		fmt.Printf("Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("Last scanned: never")
	}
	fmt.Printf("Articles: %d total, %d unread\n", stats.TotalArticles, stats.UnreadArticles)
	return nil
}

func newBlogsAddCommand() *cobra.Command {
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:   "add [name] <url>",
		Short: "Add a new blog to track.",
		Long: `Add a new blog to track.

If only a URL is provided, the blog name is automatically extracted from the feed.
If both name and URL are provided, the custom name is used.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			var name, url string
			if len(args) == 1 {
				// Only URL provided, auto-extract name
				url = args[0]
				// Try to discover feed URL if not provided
				discoveredFeedURL := feedURL
				if discoveredFeedURL == "" {
					discoveredFeedURL, _ = rss.DiscoverFeedURL(url, 30*time.Second)
				}
				if discoveredFeedURL != "" {
					extractedName, err := rss.GetFeedTitle(discoveredFeedURL, 30*time.Second)
					if err == nil && extractedName != "" {
						name = extractedName
					}
				}
				if name == "" {
					err := fmt.Errorf("Could not extract name from feed. Please provide a name.")
					printError(err)
					return markError(err)
				}
			} else {
				// Both name and URL provided
				name = args[0]
				url = args[1]
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = controller.AddBlog(db, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Added blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "RSS/Atom feed URL (auto-discovered if not provided)")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "CSS selector for HTML scraping fallback")
	return cmd
}

func newBlogsEditCommand() *cobra.Command {
	var name string
	var url string
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a tracked blog.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			blogID, err := parseID(args[0])
			if err != nil {
				printError(fmt.Errorf("Invalid blog ID: %s", args[0]))
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = controller.UpdateBlog(db, blogID, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Blog %d updated.\n", blogID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New blog name")
	cmd.Flags().StringVar(&url, "url", "", "New blog URL")
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "New feed URL")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "New scrape selector")
	return cmd
}

func newBlogsRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a blog from tracking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			blogID, err := parseID(args[0])
			if err != nil {
				printError(fmt.Errorf("Invalid blog ID: %s", args[0]))
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			// Get blog name for confirmation message
			blog, err := db.GetBlog(blogID)
			if err != nil {
				return err
			}
			if blog == nil {
				err := controller.BlogIDNotFoundError{ID: blogID}
				printError(err)
				return markError(err)
			}

			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' (ID: %d) and all its articles?", blog.Name, blogID))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			if err := controller.RemoveBlogByID(db, blogID); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s' (ID: %d)\n", blog.Name, blogID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newScanCommand() *cobra.Command {
	var silent bool
	var workers int

	cmd := &cobra.Command{
		Use:   "scan [blog_name]",
		Short: "Scan blogs for new articles.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 1 {
				result, err := scanner.ScanBlogByName(db, args[0])
				if err != nil {
					return err
				}
				if result == nil {
					err := fmt.Errorf("Blog '%s' not found", args[0])
					printError(err)
					return markError(err)
				}
				if !silent {
					printScanResult(*result)
				}
			} else {
				blogs, err := db.ListBlogs()
				if err != nil {
					return err
				}
				if len(blogs) == 0 {
					fmt.Println("No blogs tracked yet. Use 'blogwatcher add' to add one.")
					return nil
				}
				if !silent {
					color.New(color.FgCyan).Printf("Scanning %d blog(s)...\n\n", len(blogs))
				}
				results, err := scanner.ScanAllBlogs(db, workers)
				if err != nil {
					return err
				}
				totalNew := 0
				for _, result := range results {
					if !silent {
						printScanResult(result)
					}
					totalNew += result.NewArticles
				}
				if !silent {
					fmt.Println()
					if totalNew > 0 {
						color.New(color.FgGreen, color.Bold).Printf("Found %d new article(s) total!\n", totalNew)
					} else {
						color.New(color.FgYellow).Println("No new articles found.")
					}
				}
			}

			if silent {
				fmt.Println("scan done")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&silent, "silent", "s", false, "Only output 'scan done' when complete")
	cmd.Flags().IntVarP(&workers, "workers", "w", 8, "Number of concurrent workers when scanning all blogs")
	return cmd
}

func newArticlesCommand() *cobra.Command {
	var showAll bool
	var showRead bool
	var blogName string
	var blogID int64
	var page int
	var perPage int
	var fields string
	var search string

	cmd := &cobra.Command{
		Use:   "articles [id]",
		Short: "Manage articles.",
		Long: `Manage articles.

Without arguments, lists unread articles.
With an article ID, shows detailed information about that article.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			// If an ID is provided, show article details
			if len(args) == 1 {
				return runArticlesShow(db, args[0])
			}

			// Build blogID pointer
			var blogIDPtr *int64
			if blogID > 0 {
				blogIDPtr = &blogID
			}

			// Otherwise, list articles
			return runArticlesList(db, showAll, showRead, blogName, blogIDPtr, search, page, perPage, fields)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().BoolVarP(&showRead, "read", "r", false, "Show only read articles")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")
	cmd.Flags().Int64Var(&blogID, "blog-id", 0, "Filter by blog ID")
	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Articles per page (max 100)")
	cmd.Flags().StringVar(&fields, "fields", "id,title,blog,read,url,published", "Comma-separated fields to display")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Filter by article title (partial match)")

	cmd.AddCommand(newArticlesReadCommand())
	cmd.AddCommand(newArticlesUnreadCommand())
	cmd.AddCommand(newArticlesReadAllCommand())

	return cmd
}

func runArticlesList(db *storage.Database, showAll bool, showRead bool, blogName string, blogID *int64, search string, page int, perPage int, fields string) error {
	// Determine status filter
	status := "unread"
	if showAll {
		status = "all"
	} else if showRead {
		status = "read"
	}

	// Validate and normalize pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := controller.GetArticlesWithSearch(db, status, blogName, blogID, search, page, perPage)
	if err != nil {
		printError(err)
		return markError(err)
	}

	if result.Total == 0 {
		label := "Unread articles"
		if status == "read" {
			label = "Read articles"
		} else if status == "all" {
			label = "Articles"
		}
		msg := fmt.Sprintf("%s (no results)", label)
		if search != "" {
			msg = fmt.Sprintf("%s matching '%s' (no results)", label, search)
		}
		color.New(color.FgCyan, color.Bold).Println(msg)
		return nil
	}

	// Parse fields
	fieldList := parseFields(fields)

	label := "Unread articles"
	if status == "read" {
		label = "Read articles"
	} else if status == "all" {
		label = "All articles"
	}
	if search != "" {
		label = fmt.Sprintf("%s matching '%s'", label, search)
	}
	color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)
	for _, article := range result.Articles {
		output := FormatArticleFields(&article, result.BlogNames, fieldList)
		fmt.Printf("  %s\n", output)
	}
	return nil
}

func runArticlesShow(db *storage.Database, idStr string) error {
	articleID, err := parseID(idStr)
	if err != nil {
		printError(err)
		return markError(err)
	}

	article, err := db.GetArticle(articleID)
	if err != nil {
		printError(err)
		return markError(err)
	}
	if article == nil {
		err := fmt.Errorf("Article %d not found", articleID)
		printError(err)
		return markError(err)
	}

	blog, err := db.GetBlog(article.BlogID)
	if err != nil {
		return err
	}

	blogNames := make(map[int64]string)
	if blog != nil {
		blogNames[article.BlogID] = blog.Name
	}

	// Default detail fields
	fields := []string{"id", "title", "url", "blog", "published", "discovered", "read", "content"}
	output := FormatArticleDetail(article, blogNames, fields)
	fmt.Println(output)
	return nil
}

func newArticlesReadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <article_id>",
		Short: "Mark an article as read.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleRead(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if article.IsRead {
				fmt.Printf("Article %d is already marked as read.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as read\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newArticlesUnreadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread <article_id>",
		Short: "Mark an article as unread.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleUnread(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if !article.IsRead {
				fmt.Printf("Article %d is already marked as unread.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as unread\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newArticlesReadAllCommand() *cobra.Command {
	var blogName string
	var yes bool

	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark all unread articles as read.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := controller.GetArticles(db, "unread", blogName, 1, 1000, "")
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(result.Articles) == 0 {
				color.New(color.FgGreen).Println("No unread articles to mark as read.")
				return nil
			}

			if !yes {
				scope := "all blogs"
				if blogName != "" {
					scope = fmt.Sprintf("from '%s'", blogName)
				}
				confirmed, err := confirm(fmt.Sprintf("Mark %d article(s) %s as read?", len(result.Articles), scope))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			marked, err := controller.MarkAllArticlesRead(db, blogName)
			if err != nil {
				printError(err)
				return markError(err)
			}

			color.New(color.FgGreen).Printf("Marked %d article(s) as read\n", len(marked))
			return nil
		},
	}

	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Only mark articles from this blog")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func parseFields(fields string) []string {
	if fields == "" {
		return []string{"id", "title", "blog", "read", "url", "published"}
	}
	parts := strings.Split(fields, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func newImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.opml>",
		Short: "Import blogs from an OPML file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			opmlDoc, err := opml.ParseFile(filePath)
			if err != nil {
				printError(err)
				return markError(err)
			}

			outlines := opml.ExtractOutlines(opmlDoc)

			if len(outlines) == 0 {
				color.New(color.FgYellow).Println("No feeds found in OPML file.")
				return nil
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			color.New(color.FgCyan).Printf("Importing from %s...\n\n", filePath)

			result := controller.ImportBlogs(db, outlines)

			for _, imported := range result.Imported {
				color.New(color.FgGreen).Printf("✓ Imported: %q\n", imported.Name)
			}
			for _, skipped := range result.Skipped {
				color.New(color.FgYellow).Printf("⚠ Skipped %q: %s\n", skipped.Name, skipped.Reason)
			}
			for _, failed := range result.Failed {
				name := failed.Name
				if name == "" {
					name = "(unknown)"
				}
				color.New(color.FgRed).Printf("✗ Failed: %q - %s\n", name, failed.Reason)
			}

			fmt.Println()
			color.New(color.FgCyan, color.Bold).Printf(
				"Import complete: %d imported, %d skipped, %d failed\n",
				len(result.Imported),
				len(result.Skipped),
				len(result.Failed),
			)

			return nil
		},
	}
	return cmd
}

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize blogwatcher configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := config.DefaultDataDir()
			if err != nil {
				return fmt.Errorf("get data directory: %w", err)
			}

			configPath, err := config.DefaultConfigPath()
			if err != nil {
				return fmt.Errorf("get config path: %w", err)
			}

			// Check if already initialized
			existingCfg, err := config.Load("")
			if err != nil {
				return err
			}
			if existingCfg.IsConfigured() {
				color.New(color.FgYellow).Printf("Configuration already exists at %s\n", configPath)
				return nil
			}

			color.New(color.FgCyan, color.Bold).Println("Welcome to BlogWatcher!")
			fmt.Println()
			color.New(color.FgWhite).Println("Let's set up your configuration.")
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)

			// Base URL
			fmt.Printf("API Base URL [%s]: ", config.DefaultBaseURL)
			baseURL, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			baseURL = strings.TrimSpace(baseURL)
			if baseURL == "" {
				baseURL = config.DefaultBaseURL
			}

			// API Key
			fmt.Print("API Key: ")
			apiKey, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" {
				color.New(color.FgRed).Println("API Key is required.")
				return fmt.Errorf("API key is required")
			}

			// Model
			fmt.Printf("Model [%s]: ", config.DefaultModel)
			modelName, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				modelName = config.DefaultModel
			}

			// Create config
			cfg := &config.Config{
				LLM: config.LLMConfig{
					BaseURL: baseURL,
					APIKey:  apiKey,
					Model:   modelName,
				},
			}

			// Save config
			if err := cfg.Save(""); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			// Initialize database
			db, err := storage.OpenDatabase("")
			if err != nil {
				return fmt.Errorf("initialize database: %w", err)
			}
			db.Close()

			fmt.Println()
			color.New(color.FgGreen, color.Bold).Println("✓ Configuration saved!")
			color.New(color.FgWhite).Printf("  Config: %s\n", configPath)
			color.New(color.FgWhite).Printf("  Database: %s\n", dataDir+"/blogwatcher.db")
			fmt.Println()
			color.New(color.FgCyan).Println("You can now use 'blogwatcher add' to start tracking blogs.")

			return nil
		},
	}
	return cmd
}

func newSummaryCommand() *cobra.Command {
	var allFlag bool
	var forceFlag bool
	var daysFlag int

	cmd := &cobra.Command{
		Use:   "summary [article_id]",
		Short: "Generate LLM summary for articles.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.Load("")
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if !cfg.IsConfigured() {
				color.New(color.FgRed).Println("Not configured. Run 'blogwatcher init' first.")
				return fmt.Errorf("not configured")
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			llmClient := llm.NewClient(cfg.LLM)

			ctx := cmd.Context()

			if allFlag {
				return runSummaryAll(ctx, db, llmClient, forceFlag, daysFlag)
			}

			if len(args) == 0 {
				return fmt.Errorf("article_id is required unless --all is specified")
			}

			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}

			return runSummarySingle(ctx, db, llmClient, articleID, forceFlag)
		},
	}

	cmd.Flags().BoolVar(&allFlag, "all", false, "Generate summaries for all articles without one")
	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Regenerate even if summary exists")
	cmd.Flags().IntVarP(&daysFlag, "days", "d", 0, "Only process articles discovered within the last N days (use with --all)")
	return cmd
}

func runSummarySingle(ctx context.Context, db *storage.Database, client *llm.Client, articleID int64, force bool) error {
	article, err := controller.GenerateSummary(ctx, db, client, articleID, force)
	if err != nil {
		printError(err)
		return markError(err)
	}

	fmt.Printf("Title: %s\n\n", article.Title)
	fmt.Printf("Summary: %s\n", article.Summary)
	return nil
}

func runSummaryAll(ctx context.Context, db *storage.Database, client *llm.Client, force bool, days int) error {
	articles, err := db.ListArticles(nil, nil, days, 1, storage.NoPagination, "")
	if err != nil {
		return err
	}

	var toProcess []model.Article
	for _, a := range articles {
		if a.Summary == "" || force {
			toProcess = append(toProcess, a)
		}
	}

	if len(toProcess) == 0 {
		color.New(color.FgGreen).Println("All articles already have summaries.")
		return nil
	}

	generated := 0
	skipped := 0
	failed := 0

	for _, article := range toProcess {
		_, err := controller.GenerateSummary(ctx, db, client, article.ID, force)
		if err != nil {
			var noContentErr llm.NoContentError
			if errors.As(err, &noContentErr) {
				color.New(color.FgYellow).Printf("⚠ Article %d: No content available\n", article.ID)
				skipped++
			} else {
				color.New(color.FgRed).Printf("✗ Article %d: %s\n", article.ID, err.Error())
				failed++
			}
		} else {
			generated++
		}
	}

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("Complete: %d generated, %d skipped, %d failed\n", generated, skipped, failed)
	return nil
}

func printScanResult(result scanner.ScanResult) {
	statusColor := color.FgWhite
	if result.NewArticles > 0 {
		statusColor = color.FgGreen
	}
	color.New(color.FgWhite, color.Bold).Printf("  %s\n", result.BlogName)
	if result.Error != "" {
		color.New(color.FgRed).Printf("    Error: %s\n", result.Error)
		return
	}
	if result.Source == "none" {
		color.New(color.FgYellow).Println("    No feed or scraper configured")
		return
	}
	sourceLabel := "HTML"
	if result.Source == "rss" {
		sourceLabel = "RSS"
	}
	fmt.Printf("    Source: %s | Found: %d | ", sourceLabel, result.TotalFound)
	color.New(statusColor).Printf("New: %d\n", result.NewArticles)
}

func printError(err error) {
	color.New(color.FgRed).Printf("Error: %s\n", err.Error())
}

func parseID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid article id: %s", value)
	}
	return parsed, nil
}

func confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.AddTemplateFunc("now", func() string { return time.Now().Format(time.RFC3339) })
}
