terraform {
  required_providers {
    oncallalert = {
      source = "OnCallAlert/oncallalert"
    }
  }
}

# Point the provider at your own server. The token needs Admin scope
# (profile page -> API Tokens). Both can also come from ONCALLALERT_URL / ONCALLALERT_TOKEN.
provider "oncallalert" {
  url   = "https://oncall.example.com"
  token = var.oncallalert_token
}

variable "oncallalert_token" {
  type      = string
  sensitive = true
}

data "oncallalert_user" "alice" {
  email = "alice@example.com"
}

data "oncallalert_user" "bob" {
  email = "bob@example.com"
}

data "oncallalert_user" "lead" {
  email = "lead@example.com"
}

resource "oncallalert_rotation" "backend" {
  name         = "Backend weekly"
  time_zone    = "Europe/Berlin"
  start        = "2026-01-05T09:00:00+01:00"
  type         = "weekly"
  shift_length = 1
  user_ids     = [data.oncallalert_user.alice.id, data.oncallalert_user.bob.id]
}

resource "oncallalert_schedule" "backend" {
  name        = "Backend"
  description = "Backend team on-call"
  time_zone   = "Europe/Berlin"
}

# The rotation is on call on weekdays during office hours.
resource "oncallalert_schedule_rule" "office_hours" {
  schedule_id = oncallalert_schedule.backend.id
  target_type = "rotation"
  target_id   = oncallalert_rotation.backend.id
  start       = "09:00"
  end         = "18:00"
  weekdays    = ["mon", "tue", "wed", "thu", "fri"]
}

resource "oncallalert_escalation_policy" "primary" {
  name        = "Primary"
  description = "Page the on-call engineer, then the whole team"
  repeat      = 2

  steps = [
    {
      delay_minutes = 5
      actions = [
        { type = "builtin-rotation", args = { rotation_id = oncallalert_rotation.backend.id } },
      ]
    },
    {
      delay_minutes = 15
      actions = [
        { type = "builtin-schedule", args = { schedule_id = oncallalert_schedule.backend.id } },
        { type = "builtin-user", args = { user_id = data.oncallalert_user.lead.id } },
      ]
    },
  ]
}

resource "oncallalert_service" "api" {
  name                 = "Public API"
  description          = "api.example.com"
  escalation_policy_id = oncallalert_escalation_policy.primary.id
}

resource "oncallalert_service_label" "team" {
  service_id = oncallalert_service.api.id
  key        = "example.com/team"
  value      = "backend"
}

resource "oncallalert_integration_key" "grafana" {
  service_id = oncallalert_service.api.id
  type       = "grafana"
  name       = "Grafana alerts"
}

# The ingest URL to paste into Grafana's contact point.
output "grafana_webhook_url" {
  value     = oncallalert_integration_key.grafana.href
  sensitive = true
}

# Bob covers for Alice over the holidays.
resource "oncallalert_user_override" "holiday" {
  schedule_id    = oncallalert_schedule.backend.id
  start          = "2026-12-24T00:00:00Z"
  end            = "2026-12-27T00:00:00Z"
  add_user_id    = data.oncallalert_user.bob.id
  remove_user_id = data.oncallalert_user.alice.id
}
