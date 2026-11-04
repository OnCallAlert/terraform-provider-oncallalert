# Contributing

Requires Go (see `go.mod`) and Terraform or OpenTofu.

```sh
make build     # bin/terraform-provider-oncallalert
make test      # unit tests: no network, no server
make lint      # go vet + gofmt
make docs      # regenerate docs/ from the schema and examples/ (commit the result)
```

## Acceptance tests

Acceptance tests (`TestAcc*`) create and destroy real objects, so run them only against a **throwaway** server.

1. Start Postgres, e.g. `docker run -d --name oca-acc-pg -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=oncallalert -p 127.0.0.1:55433:5432 postgres:17-alpine`.
2. Run an OnCallAlert server (newer than v1.0.0) against it, with a license that is not locked
   (an unlicensed server rejects alerts but API CRUD still needs a valid license state).
3. Create an admin user and an Admin-scope API token, and one more user to put on call.
4. Run:

```sh
export ONCALLALERT_URL=http://127.0.0.1:8081
export ONCALLALERT_TOKEN=...                       # Admin scope
export ONCALLALERT_TEST_USER_EMAIL=bob@example.com # an existing user
make testacc
```

Each test checks create, an empty plan afterwards, an in-place update, import, and destroy.

## Using a local build

```hcl
# ~/.terraformrc
provider_installation {
  dev_overrides {
    "OnCallAlert/oncallalert" = "/path/to/terraform-provider-oncallalert/bin"
  }
  direct {}
}
```

## Releasing

Push a tag `vX.Y.Z`. The Release workflow builds with goreleaser and signs the checksums with the GPG key in the
`GPG_PRIVATE_KEY` / `PASSPHRASE` secrets. The Terraform Registry picks up new GitHub releases automatically.
