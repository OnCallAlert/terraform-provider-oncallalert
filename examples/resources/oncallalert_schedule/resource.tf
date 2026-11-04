resource "oncallalert_schedule" "backend" {
  name        = "Backend"
  description = "Backend team on-call"
  time_zone   = "Europe/Berlin"
}
