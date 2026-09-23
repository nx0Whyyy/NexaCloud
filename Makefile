.PHONY: build test lint docker-up docker-down deps tidy

GOCMD=go
MODULES=./pkg/model ./services/orchestrator ./services/scheduler ./services/registry ./services/autoscaler ./services/deployment ./services/backup ./services/monitoring ./agent ./cli

build:
	@for module in $(MODULES); do \
		echo "building $$module"; \
		(cd $$module && $(GOCMD) build ./...); \
	done

test:
	@for module in $(MODULES); do \
		echo "testing $$module"; \
		(cd $$module && $(GOCMD) test ./...); \
	done

lint:
	@for module in $(MODULES); do \
		echo "linting $$module"; \
		(cd $$module && golangci-lint run ./...); \
	done

tidy:
	@for module in $(MODULES); do \
		echo "tidying $$module"; \
		(cd $$module && $(GOCMD) mod tidy); \
	done

docker-up:
	docker compose -f docker/compose/docker-compose.yml up -d

docker-down:
	docker compose -f docker/compose/docker-compose.yml down

deps:
	@for module in $(MODULES); do \
		echo "downloading $$module"; \
		(cd $$module && $(GOCMD) mod download); \
	done
