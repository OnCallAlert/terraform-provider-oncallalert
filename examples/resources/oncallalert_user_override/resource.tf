# Bob covers for Alice over the holidays.
resource "oncallalert_user_override" "holiday" {
  schedule_id    = oncallalert_schedule.backend.id
  start          = "2026-12-24T00:00:00Z"
  end            = "2026-12-27T00:00:00Z"
  add_user_id    = data.oncallalert_user.bob.id
  remove_user_id = data.oncallalert_user.alice.id
}
