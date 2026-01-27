.DEFAULT_GOAL := help

mod: ## Run go mod tidy
	go mod tidy

update: ## Update go mod dependencies
	go get -u
	make mod

run: ## Run the application
	go run .

test: ## Run unit tests
	# -v (verbose)
	# ./... (look for tests in all directories)
	go test -v ./...

bench: ## Run benchmarks (only)
	# -v (verbose)
	# -bench . (run all found benchmarks)
	# -benchmem (show memory allocation stats)
	# -run ^$$ (run no unit tests - only benchmarks)
	# ./... (look for benchmarks in all directories)
	go test -v -bench . -benchmem -run ^$$ ./...

coverage: ## Generate and open test coverage report
	go test -v ./... \
		-coverpkg=./... \
		-covermode=atomic \
		-coverprofile=coverage.out \
		|| true
	go tool cover \
		-html=coverage.out \
		-o coverage.html
	rm coverage.out
	open coverage.html

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'
