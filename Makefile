export GOWORK := off
TFPLUGINDOCS := github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0

.PHONY: build install test testacc docs lint

build:
	go build -o bin/terraform-provider-oncallalert .

install:
	go install .

test:
	go test -count=1 ./...

# Needs a running OnCallAlert server: ONCALLALERT_URL, ONCALLALERT_TOKEN (admin) and
# ONCALLALERT_TEST_USER_EMAIL (an existing user). See CONTRIBUTING.md.
testacc:
	TF_ACC=1 go test -count=1 -timeout 20m ./internal/provider/ -run TestAcc

docs:
	go run $(TFPLUGINDOCS) generate --provider-name oncallalert

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"
