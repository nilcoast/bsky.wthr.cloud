BINARY_NAME=bsky.wthr.cloud
REGISTRY=registry.hl1.benoist.dev
IMAGE_NAME=${REGISTRY}/${BINARY_NAME}
VERSION?=latest

build:
	GOARCH=amd64 GOOS=linux go build -o release/${BINARY_NAME} .

docker-build:
	docker build -t ${IMAGE_NAME}:${VERSION} .

docker-push: docker-build
	docker push ${IMAGE_NAME}:${VERSION}

release: docker-push
	@echo "Docker image pushed to ${IMAGE_NAME}:${VERSION}"

clean:
	go clean
	rm -f release/${BINARY_NAME}
	docker rmi ${IMAGE_NAME}:${VERSION} || true

test:
	@echo "Testing weather posts for all cities..."
	@echo "======================================"
	@set -a && . ./.env && set +a && go run main.go --city msp --dry-run
	@echo ""
	@set -a && . ./.env && set +a && go run main.go --city chicago --dry-run
	@echo ""
	@set -a && . ./.env && set +a && go run main.go --city sfo --dry-run
	@echo ""
	@set -a && . ./.env && set +a && go run main.go --city nyc --dry-run
	@echo ""
	@echo "======================================"
	@echo "All city tests completed."

coverage:
	go test ./... -coverprofile=coverage.out

dep:
	go mod download

vet:
	go vet

lint:
	golangci-lint run --enable-all
