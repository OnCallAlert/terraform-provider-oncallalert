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
