# Terraform provider for OnCallAlert

Manage your self-hosted [OnCallAlert](https://oncallalert.dev) server as code: escalation policies,
rotations, schedules, schedule rules, user overrides, services, service labels and integration keys.

The provider talks only to **your own server**, using an API token you create there. Nothing passes through
OnCallAlert's infrastructure.

Works with Terraform, OpenTofu and Pulumi (see [Pulumi](#pulumi)).

## Requirements

- An OnCallAlert server **newer than v1.0.0** (the IaC API is not in v1.0.0).
- An API token with **Admin** scope: in the web UI open your profile, then *API Tokens*, then create a token
  with scope *Admin*. Only admins can do this, and the token can never do more than its owner.
- Terraform >= 1.0 or OpenTofu >= 1.6.

## Usage

```hcl
terraform {
  required_providers {
    oncallalert = {
      source = "OnCallAlert/oncallalert"
    }
  }
}

provider "oncallalert" {
  url   = "https://oncall.example.com" # or ONCALLALERT_URL
  token = var.oncallalert_token        # or ONCALLALERT_TOKEN
}

data "oncallalert_user" "alice" {
  email = "alice@example.com"
}

resource "oncallalert_rotation" "backend" {
  name      = "Backend weekly"
  time_zone = "Europe/Berlin"
  start     = "2026-01-05T09:00:00+01:00"
  type      = "weekly"
  user_ids  = [data.oncallalert_user.alice.id]
}

resource "oncallalert_escalation_policy" "primary" {
  name = "Primary"
  steps = [{
    delay_minutes = 5
    actions = [{ type = "builtin-rotation", args = { rotation_id = oncallalert_rotation.backend.id } }]
  }]
}

resource "oncallalert_service" "api" {
  name                 = "Public API"
  escalation_policy_id = oncallalert_escalation_policy.primary.id
}
```

A full example is in [`examples/complete/main.tf`](examples/complete/main.tf). The reference for every resource
and data source is in [`docs/`](docs/) and on the Terraform Registry.

### Resources

| Resource | Import ID |
|---|---|
| `oncallalert_escalation_policy` (with ordered `steps`) | `<id>` |
| `oncallalert_rotation` | `<id>` |
| `oncallalert_schedule` | `<id>` |
| `oncallalert_schedule_rule` | `<schedule_id>/<rule_id>` |
| `oncallalert_user_override` | `<id>` |
| `oncallalert_service` | `<id>` |
| `oncallalert_service_label` | `<service_id>/<key>` |
| `oncallalert_integration_key` (exposes the ingest URL as sensitive `href`) | `<id>` |

### Data sources

`oncallalert_user` (by email), `oncallalert_service`, `oncallalert_schedule`, `oncallalert_escalation_policy`,
`oncallalert_rotation` (by exact name).

### Things to know

- Names of services, schedules, rotations and escalation policies are unique on the server. Creating one with a
  name that is already taken fails.
- Objects deleted outside Terraform are dropped from state on the next refresh and created again on apply.
- Deleting a schedule also deletes its rules and overrides on the server.

## Pulumi

Pulumi can use this provider directly. In your Pulumi project run:

```sh
pulumi package add terraform-provider OnCallAlert/oncallalert
```

This generates a local SDK for your project's language. Configure it with `oncallalert:url` and
`oncallalert:token` (`pulumi config set --secret oncallalert:token ...`), or the same `ONCALLALERT_URL` /
`ONCALLALERT_TOKEN` environment variables. See Pulumi's
[Any Terraform Provider](https://www.pulumi.com/registry/packages/terraform-provider/) docs.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MPL-2.0](LICENSE)
