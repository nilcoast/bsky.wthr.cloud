package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reiver/go-atproto/com/atproto/repo"
	"github.com/reiver/go-atproto/com/atproto/server"
)

type WthrResp struct {
	Current struct {
		Properties struct {
			Periods []struct {
				DetailedForecast string `json:"detailedForecast"`
				Name             string `json:"name"`
			} `json:"periods"`
		} `json:"properties"`
	} `json:"current"`
}

type LlamaResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LlamaReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type CityConfig struct {
	Name       string
	Latitude   float64
	Longitude  float64
	Identifier string
	EnvVar     string
	Model      string
}

// Base prompt template used for all cities
const basePromptTemplate = `You are a helpful weather posting bot. You live in %s. You only use weather reporting terms like: wind speed in mph, temperature in fahrenheit. You never make up the weather, because the actual forecast from the National Weather service is included in your context. It's labeled "Current Forecast"

Create a tweet (less than 240 characters) that shows:
1. Current conditions with emoji at start
2. Today's weather timeline using emojis (morning→afternoon→evening)
3. High/low temps
4. Key weather events

Format example: 🌧️ Now: 45°F rain | Day: 🌥️→⛈️→🌙 | Hi: 52° Lo: 38° | Heavy rain 2-5pm

Do not make up anything.
Never add hashtags.
Include date/time from "Current Date".
Use emojis to show weather progression throughout the day.

Weather emoji guide:
☀️ = sunny/clear
⛅ = partly cloudy
☁️ = cloudy
🌧️ = rain
⛈️ = thunderstorm
🌨️ = snow
❄️ = heavy snow
🌫️ = fog
💨 = windy
🌙 = clear night

Pack maximum weather info using emojis and concise text.

Current Date: %s
Current Forecast:

%s`

// HTTP helpers
func getJSON(url string, v interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("JSON decode failed: %w", err)
	}
	return nil
}

func postJSON(url string, payload interface{}, response interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON marshal failed: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("JSON decode failed: %w", err)
	}
	return nil
}

// Environment helpers
func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required", key)
	}
	return value
}

// Weather service
func fetchWeather(apiURL string, lat, lon float64) (*WthrResp, error) {
	url := fmt.Sprintf("%s?latitude=%f&longitude=%f", apiURL, lat, lon)
	var weather WthrResp
	if err := getJSON(url, &weather); err != nil {
		return nil, fmt.Errorf("fetch weather failed: %w", err)
	}
	return &weather, nil
}

// LLM service
func generatePost(apiURL, model, prompt string) (string, error) {
	req := &LlamaReq{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	log.Printf("Sending to LLM: %+v", req)

	var resp LlamaResp
	if err := postJSON(apiURL, req, &resp); err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty LLM response")
	}

	return strings.Trim(resp.Choices[0].Message.Content, "\""), nil
}

// Bluesky service
func publishToBluesky(identifier, password, text string) error {
	var sessionResp server.CreateSessionResponse
	if err := server.CreateSession(&sessionResp, identifier, password); err != nil {
		return fmt.Errorf("create session failed: %w", err)
	}

	post := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().Format("2006-01-02T15:04:05.999Z"),
	}

	if err := repo.CreateRecord(&sessionResp, sessionResp.AccessJWT, identifier, "app.bsky.feed.post", post); err != nil {
		return fmt.Errorf("create record failed: %w", err)
	}

	return nil
}

// Generate a unified prompt for all cities
func generatePrompt(cityName, date, forecast string) string {
	return fmt.Sprintf(basePromptTemplate, cityName, date, forecast)
}

// Process city weather
func processCityWeather(config CityConfig, weatherAPI, ollamaAPI string, dryRun bool) error {
	// Fetch weather
	weather, err := fetchWeather(weatherAPI, config.Latitude, config.Longitude)
	if err != nil {
		return fmt.Errorf("weather fetch for %s failed: %w", config.Name, err)
	}

	// Generate post with enhanced weather data
	now := time.Now().Format(time.RubyDate)

	// Create a comprehensive weather summary using multiple periods
	// Include current conditions, progression throughout the day, and key events
	var forecastDetails string
	if len(weather.Current.Properties.Periods) > 0 {
		// Include first few periods to show weather progression
		for i := 0; i < 4 && i < len(weather.Current.Properties.Periods); i++ {
			period := weather.Current.Properties.Periods[i]
			forecastDetails += fmt.Sprintf("Period %d (%s): %s\n",
				i+1, period.Name, period.DetailedForecast)
		}
	}

	prompt := generatePrompt(config.Name, now, forecastDetails)

	post, err := generatePost(ollamaAPI, config.Model, prompt)
	if err != nil {
		return fmt.Errorf("post generation for %s failed: %w", config.Name, err)
	}

	log.Printf("Generated post for %s: %s", config.Name, post)

	if dryRun {
		fmt.Printf("\n🔵 DRY RUN - Generated post for %s:\n", config.Name)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("%s\n", post)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Character count: %d/240\n", len(post))
		return nil
	}

	// Publish to Bluesky
	password := requireEnv(config.EnvVar)
	if err := publishToBluesky(config.Identifier, password, post); err != nil {
		return fmt.Errorf("publish for %s failed: %w", config.Name, err)
	}

	log.Printf("Successfully posted weather for %s", config.Name)
	return nil
}

var cityConfigs = map[string]CityConfig{
	"msp": {
		Name:       "Minneapolis St Paul",
		Latitude:   44.88194,
		Longitude:  -93.22167,
		Identifier: "msp.wthr.cloud",
		EnvVar:     "MSP_WTHR_BSKY_PASS",
		Model:      "mistral:7b",
	},
	"chicago": {
		Name:       "Chicago",
		Latitude:   41.975844,
		Longitude:  -87.6633969,
		Identifier: "chicago.wthr.cloud",
		EnvVar:     "CHICAGO_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
	},
	"sfo": {
		Name:       "San Francisco",
		Latitude:   37.7897,
		Longitude:  -122.3972,
		Identifier: "sfo.wthr.cloud",
		EnvVar:     "SFO_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
	},
	"nyc": {
		Name:       "New York City",
		Latitude:   40.7128,
		Longitude:  -74.0060,
		Identifier: "nyc.wthr.cloud",
		EnvVar:     "NYC_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
	},
}

func main() {
	var city string
	var dryRun bool
	flag.StringVar(&city, "city", "", "City to post weather for (msp, chicago, sfo, nyc)")
	flag.BoolVar(&dryRun, "dry-run", false, "Generate posts without publishing to Bluesky")
	flag.Parse()

	if city == "" {
		log.Fatal("City flag is required. Use --city with one of: msp, chicago, sfo, nyc")
	}

	config, exists := cityConfigs[city]
	if !exists {
		log.Fatalf("Unknown city: %s. Available cities: msp, chicago, sfo, nyc", city)
	}

	// Get required API endpoints
	weatherAPI := requireEnv("WEATHER_API_URL")
	ollamaAPI := requireEnv("OLLAMA_API_URL")

	// Process city weather
	if err := processCityWeather(config, weatherAPI, ollamaAPI, dryRun); err != nil {
		log.Fatal(err)
	}
}
