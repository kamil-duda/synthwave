mod:
	go mod tidy

update:
	go get -u
	make mod

run:
	go run .

test:
	# -v (verbose)
	# ./... (look for tests in all directories)
	go test -v ./...

bench:
	# -v (verbose)
	# -bench . (run all found benchmarks)
	# -benchmem (show memory allocation stats)
	# -run ^$$ (run no unit tests - only benchmarks)
	# ./... (look for benchmarks in all directories)
	go test -v -bench . -benchmem -run ^$$ ./...

coverage:
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