resource "oncallalert_service" "api" {
  name                 = "Public API"
  description          = "api.example.com"
  escalation_policy_id = oncallalert_escalation_policy.primary.id
}
