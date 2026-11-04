resource "oncallalert_service_label" "team" {
  service_id = oncallalert_service.api.id
  key        = "example.com/team"
  value      = "backend"
}
