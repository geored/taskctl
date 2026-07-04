.PHONY: test coverage clean-coverage

test:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out -race -count=1 ./... || { rm -f coverage.out; exit 1; }
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

clean-coverage:
	rm -f cov*.out cov*.html
