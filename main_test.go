package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupRouter creates a test router with all routes configured
func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
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

	api := r.Group("/api")
	{
		api.GET("/stories", getStories)
		api.GET("/stories/:type", getStories)
	}

	return r
}

// TestGetStoriesEndpoint tests the main stories endpoint
func TestGetStoriesEndpoint(t *testing.T) {
	router := setupRouter()

	tests := []struct {
		name             string
		endpoint         string
		expectedStatus   int
		validateResponse func(*testing.T, []Story)
	}{
		{
			name:           "Get Top Stories",
			endpoint:       "/api/stories",
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, stories []Story) {
				assert.NotEmpty(t, stories, "Stories should not be empty")
				for _, story := range stories {
					assert.NotZero(t, story.ID, "Story ID should not be zero")
					assert.NotEmpty(t, story.Title, "Story title should not be empty")
					assert.NotEmpty(t, story.SubmittedBy, "Story submitter should not be empty")
					assert.NotZero(t, story.CreatedAt, "Story creation time should not be zero")
					assert.Contains(t, story.CommentsURL, "news.ycombinator.com/item", "Comments URL should be a valid HN URL")
					assert.GreaterOrEqual(t, story.Comments, 0, "Comments count should be non-negative")
				}
			},
		},
		{
			name:           "Get Show HN Stories",
			endpoint:       "/api/stories/show",
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, stories []Story) {
				assert.NotEmpty(t, stories, "Stories should not be empty")
				for _, story := range stories {
					assert.Contains(t, story.Title, "Show HN:", "Show HN stories should have 'Show HN:' prefix")
					assert.Equal(t, "show", story.Type, "Story type should be 'show'")
				}
			},
		},
		{
			name:           "Get Ask HN Stories",
			endpoint:       "/api/stories/ask",
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, stories []Story) {
				assert.NotEmpty(t, stories, "Stories should not be empty")
				for _, story := range stories {
					assert.Contains(t, story.Title, "Ask HN:", "Ask HN stories should have 'Ask HN:' prefix")
					assert.Equal(t, "ask", story.Type, "Story type should be 'ask'")
				}
			},
		},
		{
			name:           "Invalid Story Type",
			endpoint:       "/api/stories/invalid",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.endpoint, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var stories []Story
				err := json.Unmarshal(w.Body.Bytes(), &stories)
				assert.NoError(t, err, "Should be able to unmarshal response")

				if tt.validateResponse != nil {
					tt.validateResponse(t, stories)
				}
			}
		})
	}
}

// TestCaching tests the caching functionality
func TestCaching(t *testing.T) {
	// Reset cache for testing
	cache = &StoriesCache{
		stories:    make(map[string][]Story),
		lastUpdate: make(map[string]time.Time),
	}

	router := setupRouter()

	// First request should hit the API
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/stories", nil)
	start := time.Now()
	router.ServeHTTP(w1, req1)
	firstDuration := time.Since(start)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request should hit the cache
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/stories", nil)
	start = time.Now()
	router.ServeHTTP(w2, req2)
	secondDuration := time.Since(start)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.True(t, secondDuration < firstDuration, "Cached request should be faster")

	// Verify responses are identical
	assert.Equal(t, w1.Body.String(), w2.Body.String(), "Cached response should match original")
}

// TestErrorResponse tests error handling
func TestErrorResponse(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stories/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err, "Should be able to unmarshal error response")
	assert.Contains(t, response.Error, "invalid story type", "Error message should indicate invalid type")
}

// TestCORSHeaders tests CORS headers
func TestCORSHeaders(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/stories", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Origin, Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
}
