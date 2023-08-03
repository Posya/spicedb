goModules = "." "e2e" "tools/analyzers"



wasm:
	GOOS=js GOARCH=wasm go build -o dist/development.wasm ./pkg/development/wasm/...

testimage: 
	DOCKER_BUILDKIT=1 docker build -t authzed/spicedb:ci .

tidy:
	for m in $(goModules); do cd $(m); \
		go mod tidy; \
	done

update:
	for m in $(goModules); do cd $(m); \
		go get -u -t -tags ci,tools ./...; \
		go mod tidy; \
	done
	

gen_go:
	go generate ./...

gen_proto:
	go run github.com/bufbuild/buf/cmd/buf generate -o pkg/proto proto/internal --template buf.gen.yaml

lint_all: lint_go lint_extra

lint_extra: lint_markdown lint_yaml

lint_yaml: 
	docker run --rm -v $(pwd):/src:ro cytopia/yamllint:1 -c /src/.yamllint /src

lint_markdown:
	docker run --rm -v $(pwd):/src:ro ghcr.io/igorshubovych/markdownlint-cli:v0.34.0 --config /src/.markdownlint.yaml /src

lint_go: 	lint_gofumpt lint_golangcilint lint_analyzers lint_vulncheck

lint_gofumpt:
	go run mvdan.cc/gofumpt -l -w .

lint_golangcilint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run --fix

lint_analyzers:
	go run ./cmd/analyzers/main.go -nilvaluecheck \
		-nilvaluecheck.skip-pkg=github.com/authzed/spicedb/pkg/proto/dispatch/v1 \
		-nilvaluecheck.disallowed-nil-return-type-paths=*github.com/authzed/spicedb/pkg/proto/dispatch/v1.DispatchCheckResponse,*github.com/authzed/spicedb/pkg/proto/dispatch/v1.DispatchExpandResponse,*github.com/authzed/spicedb/pkg/proto/dispatch/v1.DispatchLookupResponse \
		-exprstatementcheck \
		-exprstatementcheck.disallowed-expr-statement-types=*github.com/rs/zerolog.Event:MarshalZerologObject:missing Send or Msg on zerolog log Event 
		-closeafterusagecheck -closeafterusagecheck.must-be-closed-after-usage-types=github.com/authzed/spicedb/pkg/datastore.RelationshipIterator \
		-closeafterusagecheck.skip-pkg=github.com/authzed/spicedb/pkg/datastore,github.com/authzed/spicedb/internal/datastore,github.com/authzed/spicedb/internal/testfixtures \
		-paniccheck \
		-paniccheck.skip-files=_test,zz_ \
		github.com/authzed/spicedb/...

lint_vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck ./...



