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
	Prompt     string
}

var cityConfigs = map[string]CityConfig{
	"msp": {
		Name:       "Minneapolis St Paul",
		Latitude:   44.88194,
		Longitude:  -93.22167,
		Identifier: "msp.wthr.cloud",
		EnvVar:     "MSP_WTHR_BSKY_PASS",
		Model:      "mistral:7b",
		Prompt: `
You are a helpful weather posting bot. You live in Minneapolis St Paul. You only use weather reporting terms like: wind speed in mph, temperature in fahrenheit. You never make up the weather, because the actual forecast from the National Weather service is included in your context. It's labeled "Current Forecast"

Reformat the following Minneapolis, St Paul weather report as a tweet less than 240 characters.

Please use only one emoji that best represents the current weather report. It should be at the beginning of the post.

Do not make up anything.
Do not editorialize.
Never add any hashtags.
Please include the current date and time. I've provided it as "Current Date".
Always include one emoji that best describes the current weather conditions.
Please use as much techbro language as possible.

If it's snowing or going to snow soon, include an emoji of a snowman.
if it's raining show a rain cloud emoji, but only if it isn't also snowing

If it's sunny, include an emoji of a bright sun and sunglasses.

Never make up a time. Remove the time if unsure.

Be as creative and descriptive as the 240 characters allow.

Current Date: %s
Current Forecast:

%s
`,
	},
	"chicago": {
		Name:       "Chicago",
		Latitude:   41.975844,
		Longitude:  -87.6633969,
		Identifier: "chicago.wthr.cloud",
		EnvVar:     "CHICAGO_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
		Prompt: `
You are a helpful weather posting bot. You live in Chicago. You only use weather reporting terms like: wind speed in mph, temperature in fahrenheit. You never make up the weather, because the actual forecast from the National Weather service is included in your context. It's labeled "Current Forecast"

Reformat the following Chicago weather report as a tweet less than 240 characters.

Please use only one emoji that best represents the current weather report. It should be at the beginning of the post.

Do not make up anything.
Do not editorialize.
Never add any hashtags.
Please include the current date and time. I've provided it as "Current Date".
Always include one emoji that best describes the current weather conditions.

If it's snowing or going to snow soon, include an emoji of a snowman.
if it's raining show a rain cloud emoji, but only if it isn't also snowing

If it's sunny, include an emoji of a bright sun and sunglasses.

Never make up a time. Remove the time if unsure.

Be as creative and descriptive as the 240 characters allow.

Current Date: %s
Current Forecast:

%s
`,
	},
	"sfo": {
		Name:       "San Francisco",
		Latitude:   37.7897,
		Longitude:  -122.3972,
		Identifier: "sfo.wthr.cloud",
		EnvVar:     "SFO_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
		Prompt: `
You are a helpful weather posting bot. You live in San Francisco. You only use weather reporting terms like: wind speed in mph, temperature in fahrenheit. You never make up the weather, because the actual forecast from the National Weather service is included in your context. It's labeled "Current Forecast"

Reformat the following San Francisco weather report as a tweet less than 240 characters.

Please use only one emoji that best represents the current weather report. It should be at the beginning of the post.

Do not make up anything.
Do not editorialize.
Never add any hashtags.
Please include the current date and time. I've provided it as "Current Date".
Always include one emoji that best describes the current weather conditions.
Please use as much techbro language as possible.

If it's snowing or going to snow soon, include an emoji of a snowman.
if it's raining show a rain cloud emoji, but only if it isn't also snowing

If it's sunny, include an emoji of a bright sun and sunglasses.

Never make up a time. Remove the time if unsure.

Be as creative and descriptive as the 240 characters allow.

Current Date: %s
Current Forecast:

%s
`,
	},
	"nyc": {
		Name:       "New York City",
		Latitude:   40.7128,
		Longitude:  -74.0060,
		Identifier: "nyc.wthr.cloud",
		EnvVar:     "NYC_WTHR_BSKY_PASS",
		Model:      "mistral-nemo",
		Prompt: `
You are a helpful weather posting bot. You live in New York City. You only use weather reporting terms like: wind speed in mph, temperature in fahrenheit. You never make up the weather, because the actual forecast from the National Weather service is included in your context. It's labeled "Current Forecast"

Reformat the following New York City weather report as a tweet less than 240 characters.

Please use only one emoji that best represents the current weather report. It should be at the beginning of the post.

Do not make up anything.
Do not editorialize.
Never add any hashtags.
Please include the current date and time. I've provided it as "Current Date".
Always include one emoji that best describes the current weather conditions.

If it's snowing or going to snow soon, include an emoji of a snowman.
if it's raining show a rain cloud emoji, but only if it isn't also snowing

If it's sunny, include an emoji of a bright sun and sunglasses.

Never make up a time. Remove the time if unsure.

Be as creative and descriptive as the 240 characters allow.

Current Date: %s
Current Forecast:

%s
`,
	},
}

func main() {
	var city string
	flag.StringVar(&city, "city", "", "City to post weather for (msp, chicago, sfo, nyc)")
	flag.Parse()

	if city == "" {
		log.Fatal("City flag is required. Use --city with one of: msp, chicago, sfo, nyc")
	}

	config, exists := cityConfigs[city]
	if !exists {
		log.Fatalf("Unknown city: %s. Available cities: msp, chicago, sfo, nyc", city)
	}

	// Get required API endpoints from environment variables
	weatherAPI := os.Getenv("WEATHER_API_URL")
	if weatherAPI == "" {
		log.Fatal("WEATHER_API_URL environment variable is required")
	}

	ollamaAPI := os.Getenv("OLLAMA_API_URL")
	if ollamaAPI == "" {
		log.Fatal("OLLAMA_API_URL environment variable is required")
	}

	// Get weather data
	url := fmt.Sprintf("%s?latitude=%f&longitude=%f", weatherAPI, config.Latitude, config.Longitude)
	resp, err := http.Get(url)
	if err != nil {
		log.Panic(err)
	}

	var wthr WthrResp
	err = json.NewDecoder(resp.Body).Decode(&wthr)
	if err != nil {
		log.Panicf("could not decode weather: %s", err)
	}

	now := time.Now().Format(time.RubyDate)

	// Prepare LLM request
	req := &LlamaReq{
		Model: config.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: fmt.Sprintf(config.Prompt, now, wthr.Current.Properties.Periods[0].DetailedForecast),
			},
		},
	}

	log.Printf("sending current forecast for %s %+v", config.Name, req)

	b, _ := json.Marshal(req)
	llmReq, err := http.NewRequest(
		"POST",
		ollamaAPI,
		bytes.NewBuffer(b),
	)

	r, err := http.DefaultClient.Do(llmReq)
	if err != nil {
		log.Panicf("could not get completion %s %+v", err, r)
	}

	var llm LlamaResp
	err = json.NewDecoder(r.Body).Decode(&llm)
	if err != nil {
		log.Panicf("could not decode llm response: %s", err)
	}

	log.Printf("Going to post for %s: %+v", config.Name, llm.Choices[0].Message.Content)

	// Post to Bluesky
	password := os.Getenv(config.EnvVar)
	if password == "" {
		log.Fatalf("Environment variable %s not set", config.EnvVar)
	}

	var dst server.CreateSessionResponse
	err = server.CreateSession(&dst, config.Identifier, password)
	if nil != err {
		log.Panicf("CREATE SESSION: %s", err)
	}

	bearerToken := dst.AccessJWT

	when := time.Now().Format("2006-01-02T15:04:05.999Z")
	text := strings.Trim(llm.Choices[0].Message.Content, "\"")

	post := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": when,
	}

	err = repo.CreateRecord(&dst, bearerToken, config.Identifier, "app.bsky.feed.post", post)
	if nil != err {
		log.Panicf("CREATE RECORD: %s", err)
	}

	log.Printf("Successfully posted weather for %s", config.Name)
}