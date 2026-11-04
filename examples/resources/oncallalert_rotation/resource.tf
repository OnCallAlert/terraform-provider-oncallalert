resource "oncallalert_rotation" "backend" {
  name         = "Backend weekly"
  time_zone    = "Europe/Berlin"
  start        = "2026-01-05T09:00:00+01:00"
  type         = "weekly"
  shift_length = 1
  user_ids     = [data.oncallalert_user.alice.id, data.oncallalert_user.bob.id]
}
