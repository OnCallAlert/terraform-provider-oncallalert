# The rotation is on call on weekdays during office hours.
resource "oncallalert_schedule_rule" "office_hours" {
  schedule_id = oncallalert_schedule.backend.id
  target_type = "rotation"
  target_id   = oncallalert_rotation.backend.id
  start       = "09:00"
  end         = "18:00"
  weekdays    = ["mon", "tue", "wed", "thu", "fri"]
}
