project = excel-templater

pwd = $(shell pwd)
module = $(shell go list -m)

formatting:
	go fmt ./...
	go install github.com/daixiang0/gci@latest	
	gci write --skip-generated -s standard -s default -s "prefix($(module))" .

linter:
	docker run --rm -v $(pwd):/app -w /app golangci/golangci-lint:v1.49.0 golangci-lint run -v