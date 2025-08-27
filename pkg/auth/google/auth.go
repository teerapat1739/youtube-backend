package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GetOAuthConfig returns Google OAuth configuration with current environment variables
func GetOAuthConfig() *oauth2.Config {
	// Check for explicit redirect URL first (for local development)
	redirectURL := os.Getenv("REDIRECT_URL")
	if redirectURL == "" {
		// Fall back to BASE_URL for production
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			// Use the deployed URL as default for Cloud Run
			baseURL = "https://youtube-backend-283958071703.asia-southeast1.run.app"
		}
		redirectURL = baseURL + "/auth/google/callback"
	}

	return &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/youtube",
			"https://www.googleapis.com/auth/youtube.force-ssl",
		},
		Endpoint: google.Endpoint,
	}
}

// User represents a Google user
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// YouTubeSubscription represents a YouTube subscription
type YouTubeSubscription struct {
	ID      string `json:"id"`
	Channel struct {
		ID          string `json:"resourceId.channelId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Thumbnail   string `json:"thumbnails.default.url"`
	} `json:"snippet"`
}

// YouTubeSubscriptionsResponse represents the API response
type YouTubeSubscriptionsResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			ResourceId  struct {
				ChannelId string `json:"channelId"`
			} `json:"resourceId"`
			Thumbnails struct {
				Default struct {
					URL string `json:"url"`
				} `json:"default"`
			} `json:"thumbnails"`
		} `json:"snippet"`
	} `json:"items"`
	NextPageToken string `json:"nextPageToken"`
	PageInfo      struct {
		TotalResults int `json:"totalResults"`
	} `json:"pageInfo"`
}

// LoginHandler redirects to Google's OAuth page
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	config := GetOAuthConfig()

	// Debug: Print config values
	clientID := config.ClientID
	if clientID == "" {
		http.Error(w, "GOOGLE_CLIENT_ID not set", http.StatusInternalServerError)
		return
	}

	// Configure OAuth to request offline access and force consent to guarantee refresh token
	url := config.AuthCodeURL("state", 
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("approval_prompt", "force"))
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// CallbackHandler handles the callback from Google and redirects to Vue frontend
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Get frontend URL from environment variable with fallback
	frontendURL := os.Getenv("FRONTEND_URL")
	log.Println("frontendURL", frontendURL)
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Get the authorization code from URL
	code := r.URL.Query().Get("code")
	if code == "" {
		// Redirect to frontend with error
		http.Redirect(w, r, frontendURL+"/?error=no_code", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for token
	config := GetOAuthConfig()
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		http.Redirect(w, r, frontendURL+"/?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Redirect to Vue frontend with the access token using proper URL construction
	baseURL, err := url.Parse(frontendURL + "/")
	if err != nil {
		log.Printf("❌ Failed to parse frontend URL: %v", err)
		http.Redirect(w, r, frontendURL+"/?error=url_construction_failed", http.StatusTemporaryRedirect)
		return
	}

	// Create query parameters
	params := url.Values{}
	params.Add("token", token.AccessToken)

	// Set the query parameters
	baseURL.RawQuery = params.Encode()
	redirectURL := baseURL.String()

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// GetUserInfo gets user information from Google API (exported version)
func GetUserInfo(accessToken string) (*User, error) {
	return getUserInfo(accessToken)
}

// GetYouTubeSubscriptions gets user's YouTube subscriptions (exported version)
func GetYouTubeSubscriptions(accessToken string) (*YouTubeSubscriptionsResponse, error) {
	return getYouTubeSubscriptions(accessToken)
}

// getUserInfo gets user information from Google API
func getUserInfo(accessToken string) (*User, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user User
	err = json.Unmarshal(body, &user)
	return &user, err
}

// getYouTubeSubscriptions gets user's YouTube subscriptions
func getYouTubeSubscriptions(accessToken string) (*YouTubeSubscriptionsResponse, error) {
	url := "https://www.googleapis.com/youtube/v3/subscriptions?part=snippet&mine=true&maxResults=50&access_token=" + accessToken

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var subscriptions YouTubeSubscriptionsResponse
	err = json.Unmarshal(body, &subscriptions)
	return &subscriptions, err
}

// generateSubscriptionsHTML generates HTML for subscriptions list
func generateSubscriptionsHTML(subscriptions *YouTubeSubscriptionsResponse) string {
	if len(subscriptions.Items) == 0 {
		return "<p>No subscriptions found. Subscribe to some YouTube channels first!</p>"
	}

	html := ""
	for _, sub := range subscriptions.Items {
		thumbnailURL := sub.Snippet.Thumbnails.Default.URL
		if thumbnailURL == "" {
			thumbnailURL = "https://via.placeholder.com/50x50?text=YT"
		}

		description := sub.Snippet.Description
		// Handle encoding issues and limit description length
		if description == "" {
			description = "No description available"
		} else if len(description) > 100 {
			// Use rune counting for proper Unicode handling
			runes := []rune(description)
			if len(runes) > 100 {
				description = string(runes[:100]) + "..."
			}
		}

		// Escape HTML characters to prevent encoding issues
		title := htmlEscape(sub.Snippet.Title)
		description = htmlEscape(description)
		channelId := sub.Snippet.ResourceId.ChannelId

		html += fmt.Sprintf(`
			<div class="subscription">
				<img src="%s" alt="%s">
				<div class="subscription-info">
					<h3>%s</h3>
					<p><strong>Channel ID:</strong> %s</p>
					<p><strong>Description:</strong> %s</p>
					<p><a href="https://www.youtube.com/channel/%s" target="_blank">🔗 Visit Channel</a></p>
				</div>
			</div>
		`, thumbnailURL, title, title, channelId, description, channelId)
	}

	return html
}

// htmlEscape escapes HTML characters to prevent encoding issues
func htmlEscape(s string) string {
	s = fmt.Sprintf("%q", s)
	s = s[1 : len(s)-1] // Remove quotes added by %q
	return s
}
