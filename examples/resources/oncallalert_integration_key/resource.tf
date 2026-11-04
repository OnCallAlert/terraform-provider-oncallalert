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
