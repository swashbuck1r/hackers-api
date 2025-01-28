package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Story represents a Hacker News story item
type Story struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Points      int       `json:"points"`
	SubmittedBy string    `json:"submitted_by"`
	CreatedAt   time.Time `json:"created_at"`
	CommentsURL string    `json:"comments_url"`
	Type        string    `json:"type"` // "top", "show", "ask"
}

// HNItem represents the raw Hacker News API response
type HNItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
}

// Cache structure
type StoriesCache struct {
	stories    map[string][]Story
	lastUpdate map[string]time.Time
	mutex      sync.RWMutex
}

const (
	cacheExpiration = 5 * time.Minute
	maxStories      = 30
)

var cache = &StoriesCache{
	stories:    make(map[string][]Story),
	lastUpdate: make(map[string]time.Time),
}

func (sc *StoriesCache) get(storyType string) ([]Story, bool) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	lastUpdate, ok := sc.lastUpdate[storyType]
	if !ok {
		return nil, false
	}

	if time.Since(lastUpdate) > cacheExpiration {
		return nil, false
	}

	stories, ok := sc.stories[storyType]
	return stories, ok
}

func (sc *StoriesCache) set(storyType string, stories []Story) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.stories[storyType] = stories
	sc.lastUpdate[storyType] = time.Now()
}

func fetchStories(storyType string) ([]Story, error) {
	// Check cache first
	if stories, ok := cache.get(storyType); ok {
		return stories, nil
	}

	var endpoint string
	switch storyType {
	case "top":
		endpoint = "topstories"
	case "show":
		endpoint = "showstories"
	case "ask":
		endpoint = "askstories"
	default:
		return nil, fmt.Errorf("invalid story type: %s", storyType)
	}

	resp, err := http.Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/%s.json", endpoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var storyIDs []int
	if err := json.NewDecoder(resp.Body).Decode(&storyIDs); err != nil {
		return nil, err
	}

	if len(storyIDs) > maxStories {
		storyIDs = storyIDs[:maxStories]
	}

	stories := make([]Story, 0, len(storyIDs))
	for _, id := range storyIDs {
		item, err := fetchItem(id)
		if err != nil {
			continue
		}

		// Skip items that don't match the requested type
		if storyType == "show" && !strings.HasPrefix(item.Title, "Show HN:") {
			continue
		}
		if storyType == "ask" && !strings.HasPrefix(item.Title, "Ask HN:") {
			continue
		}

		story := Story{
			ID:          item.ID,
			Title:       item.Title,
			URL:         item.URL,
			Points:      item.Score,
			SubmittedBy: item.By,
			CreatedAt:   time.Unix(item.Time, 0),
			CommentsURL: fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID),
			Type:        storyType,
		}
		stories = append(stories, story)
	}

	// Update cache
	cache.set(storyType, stories)
	return stories, nil
}

func fetchItem(id int) (*HNItem, error) {
	resp, err := http.Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var item HNItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	return &item, nil
}

func getStories(c *gin.Context) {
	storyType := c.Param("type")
	if storyType == "" {
		storyType = "top"
	}

	stories, err := fetchStories(storyType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stories)
}

func main() {
	r := gin.Default()

	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API routes
	api := r.Group("/api")
	{
		api.GET("/stories", getStories)       // Default to top stories
		api.GET("/stories/:type", getStories) // Get stories by type (top/show/ask)
	}

	r.Run(":8080")
}
