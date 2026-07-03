.PHONY: coverage clean-coverage

coverage:
	go test -coverprofile=coverage.out -race -count=1 ./...
	go tool cover -html=coverage.out -o coverage.html

clean-coverage:
	rm -f coverage*.out coverage*.html cov_*.out
