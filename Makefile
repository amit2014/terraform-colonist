.PHONY: all
all: colony tvm

.PHONY: clean
clean:
	rm -f ./colony
	rm -f ./tvm

colony: vendor
	go build github.com/uber/terraform-colonist/colonist/cli/colony

.PHONY: install
install:
	go install github.com/uber/terraform-colonist/colonist/cli/colony
	go install github.com/uber/terraform-colonist/colonist/tvm/cli/tvm

.PHONY: test
test: vendor
	go test -timeout 1m -coverprofile=.coverage.out ./... \
		|grep -v -E '^\?'

tvm: vendor
	go build github.com/uber/terraform-colonist/colonist/tvm/cli/tvm

vendor:
	dep ensure
