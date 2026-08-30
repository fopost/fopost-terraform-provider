default: build

build:
	go build -v ./...

install:
	go install -v ./...

test:
	go test -race ./...

# Runs the acceptance tests against the in-process fake FoPost API. Needs a
# terraform binary on PATH; nothing reaches the real service.
testacc:
	TF_ACC=1 go test ./... -v -timeout 15m

lint:
	gofmt -l .
	go vet ./...

# Regenerates docs/ from the schemas and examples/. Needs a terraform binary.
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.23.0 generate \
		--provider-name fopost --rendered-provider-name FoPost

.PHONY: build install test testacc lint docs
