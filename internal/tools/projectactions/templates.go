package projectactions

// makefileTemplates contains language-specific Makefile templates
// All templates use tabs for indentation per Unix conventions
var makefileTemplates = map[string]string{
	"python": `.PHONY: default fix format lint test

default: lint test

fix:
	ruff check --fix .

format:
	black .

lint:
	ruff check .

test:
	pytest
`,
	"rust": `.PHONY: default build fix format lint test

default: lint test

build:
	cargo build

fix:
	cargo clippy --fix

format:
	cargo fmt

lint:
	cargo clippy

test:
	cargo test
`,
	"go": `.PHONY: default build fix format lint test

default: lint test

build:
	go build ./...

fix:
	go fmt ./...

format:
	go fmt ./...

lint:
	golangci-lint run

test:
	go test ./...
`,
	"nodejs": `.PHONY: default fix format lint test

default: lint test

fix:
	npm run lint:fix

format:
	npm run format

lint:
	npm run lint

test:
	npm test
`,
}
