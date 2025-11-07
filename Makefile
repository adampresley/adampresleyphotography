.DEFAULT_GOAL := help
.PHONY: help

VERSION=$(shell cat ./VERSION)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	cd cmd/website && CGO_ENABLED=0 go build -ldflags="-X 'main.Version=${VERSION}'" -mod=mod -o adampresleyphotography .

run: ## Run the application
	air

docker-create-builder: ## Create a builder for multi-architecture builds. Only needed once per machine
	docker buildx create --name mybuilder --driver docker-container --bootstrap

docker-build: ## Create a tar of this application
	docker build --cache-from=adampresleyphotography:latest --tag adampresleyphotography:latest --platform linux/amd64 . 
	docker save -o adampresleyphotography-latest.tar adampresleyphotography

build-linux: ## Builds a Linux binary
	cd cmd/website && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-X 'main.Version=${VERSION}'" -mod=mod -o adampresleyphotography .

