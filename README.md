# bsky.wthr.cloud

Consolidated weather bot for posting to multiple city-specific Bluesky accounts.

## Features

- Single binary/container for multiple cities
- Configurable API endpoints
- Support for different LLM models per city
- Docker containerized deployment

## Usage

### Direct Binary
```bash
./bsky.wthr.cloud --city <city_code>
```

### Docker
```bash
docker run --rm \
  -e WEATHER_API_URL=$WEATHER_API_URL \
  -e OLLAMA_API_URL=$OLLAMA_API_URL \
  -e MSP_WTHR_BSKY_PASS=$MSP_WTHR_BSKY_PASS \
  registry.hl1.benoist.dev/bsky.wthr.cloud:latest --city msp
```

Available city codes:
- `msp` - Minneapolis St Paul
- `chicago` - Chicago
- `sfo` - San Francisco
- `nyc` - New York City

## Environment Variables

### Required API Configuration
- `WEATHER_API_URL` - Weather API endpoint URL
- `OLLAMA_API_URL` - Ollama/LLM API endpoint URL

### City-specific Authentication
Each city account requires its own environment variable:
- `MSP_WTHR_BSKY_PASS` - Password for msp.wthr.cloud
- `CHICAGO_WTHR_BSKY_PASS` - Password for chicago.wthr.cloud
- `SFO_WTHR_BSKY_PASS` - Password for sfo.wthr.cloud
- `NYC_WTHR_BSKY_PASS` - Password for nyc.wthr.cloud

## Building

### Binary
```bash
make build
```

### Docker Container
```bash
make docker-build
```

## Deploying

### Push to Registry
```bash
make release
```

This will build and push the Docker image to registry.hl1.benoist.dev/bsky.wthr.cloud:latest

### Quick Deploy Script
```bash
./build-and-push.sh
```

## Testing

### Test Container Locally
```bash
./test-container.sh
```

### Test All Cities (requires env vars)
```bash
./run_all_cities.sh
```

## Configuration

Copy the example environment file and update with your values:
```bash
cp .env.example .env
```

## Example

```bash
# Binary
export WEATHER_API_URL="https://your-weather-api.com/api/forecast"
export OLLAMA_API_URL="https://your-ollama-api.com/v1/chat/completions"
export MSP_WTHR_BSKY_PASS="your-password"
./bsky.wthr.cloud --city msp

# Docker
docker run --rm \
  -e WEATHER_API_URL="https://your-weather-api.com/api/forecast" \
  -e OLLAMA_API_URL="https://your-ollama-api.com/v1/chat/completions" \
  -e MSP_WTHR_BSKY_PASS="your-password" \
  registry.hl1.benoist.dev/bsky.wthr.cloud:latest --city msp
```

## License

MIT License - see LICENSE file for details