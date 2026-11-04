package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Acceptance tests run only with TF_ACC=1 against a throwaway server (see CONTRIBUTING.md).

var accFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"oncallalert": providerserver.NewProtocol6WithError(New("acc")()),
}

func accPreCheck(t *testing.T) {
	t.Helper()
	for _, k := range []string{"ONCALLALERT_URL", "ONCALLALERT_TOKEN", "ONCALLALERT_TEST_USER_EMAIL"} {
		if os.Getenv(k) == "" {
			t.Fatalf("%s must be set for acceptance tests", k)
		}
	}
}

func accClient(t *testing.T) *client.Client {
	t.Helper()
	c, err := client.New(client.Config{URL: os.Getenv("ONCALLALERT_URL"), Token: os.Getenv("ONCALLALERT_TOKEN")})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// stackConfig builds a full config. v selects the variant: 1 = initial, 2 = updated.
func stackConfig(name string, v int) string {
	ruleEnd, labelValue, delay, extraStep := "17:00", "backend", 5, `,
    { delay_minutes = 10, actions = [{ type = "builtin-schedule", args = { schedule_id = oncallalert_schedule.s.id } }] }`
	if v == 2 {
		ruleEnd, labelValue, delay, extraStep = "18:30", "platform", 7, ""
	}
	return fmt.Sprintf(`
data "oncallalert_user" "u" {
  email = %[2]q
}

resource "oncallalert_rotation" "r" {
  name        = "%[1]s-rot"
  description = "v%[3]d"
  time_zone   = "Asia/Kolkata"
  start       = "2026-11-05T09:00:00+05:30"
  type        = "weekly"
  user_ids    = [data.oncallalert_user.u.id]
}

resource "oncallalert_schedule" "s" {
  name      = "%[1]s-sched"
  time_zone = "UTC"
}

resource "oncallalert_schedule_rule" "sr" {
  schedule_id = oncallalert_schedule.s.id
  target_type = "rotation"
  target_id   = oncallalert_rotation.r.id
  start       = "09:00"
  end         = %[4]q
  weekdays    = ["mon", "tue", "wed", "thu", "fri"]
}

resource "oncallalert_user_override" "o" {
  schedule_id = oncallalert_schedule.s.id
  start       = "2027-01-01T00:00:00Z"
  end         = "2027-01-1%[3]dT00:00:00Z"
  add_user_id = data.oncallalert_user.u.id
}

resource "oncallalert_escalation_policy" "ep" {
  name   = "%[1]s-ep"
  repeat = %[3]d
  steps = [
    { delay_minutes = %[6]d, actions = [{ type = "builtin-rotation", args = { rotation_id = oncallalert_rotation.r.id } }] }%[7]s
  ]
}

resource "oncallalert_service" "svc" {
  name                 = "%[1]s-svc"
  escalation_policy_id = oncallalert_escalation_policy.ep.id
}

resource "oncallalert_service_label" "l" {
  service_id = oncallalert_service.svc.id
  key        = "example.com/team"
  value      = %[5]q
}

resource "oncallalert_integration_key" "k" {
  service_id = oncallalert_service.svc.id
  type       = "generic"
  name       = "%[1]s-key-v%[3]d"
}

data "oncallalert_service" "lookup" {
  name       = oncallalert_service.svc.name
  depends_on = [oncallalert_service.svc]
}
`, name, os.Getenv("ONCALLALERT_TEST_USER_EMAIL"), v, ruleEnd, labelValue, delay, extraStep)
}

func importStep(addr string, idFn resource.ImportStateIdFunc) resource.TestStep {
	return resource.TestStep{ResourceName: addr, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: idFn}
}

func joinedID(addr, first, second string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return "", fmt.Errorf("%s not in state", addr)
		}
		return rs.Primary.Attributes[first] + "/" + rs.Primary.Attributes[second], nil
	}
}

func TestAccFullStack_CreateUpdateImport(t *testing.T) {
	name := "tfacc-" + acctest.RandString(6)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{
				Config: stackConfig(name, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("oncallalert_escalation_policy.ep", "steps.#", "2"),
					resource.TestCheckResourceAttr("oncallalert_escalation_policy.ep", "steps.0.delay_minutes", "5"),
					resource.TestCheckResourceAttrSet("oncallalert_integration_key.k", "href"),
					resource.TestCheckResourceAttrPair("data.oncallalert_service.lookup", "id", "oncallalert_service.svc", "id"),
					resource.TestCheckResourceAttr("oncallalert_schedule_rule.sr", "weekdays.#", "5"),
				),
			},
			{
				Config: stackConfig(name, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("oncallalert_escalation_policy.ep", "steps.#", "1"),
					resource.TestCheckResourceAttr("oncallalert_escalation_policy.ep", "steps.0.delay_minutes", "7"),
					resource.TestCheckResourceAttr("oncallalert_escalation_policy.ep", "repeat", "2"),
					resource.TestCheckResourceAttr("oncallalert_schedule_rule.sr", "end", "18:30"),
					resource.TestCheckResourceAttr("oncallalert_service_label.l", "value", "platform"),
					resource.TestCheckResourceAttr("oncallalert_rotation.r", "description", "v2"),
				),
			},
			{ResourceName: "oncallalert_rotation.r", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"start"}},     // server returns UTC; semantic equality avoids a diff
			{ResourceName: "oncallalert_schedule.s", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"time_zone"}}, // server stores "UTC" as "Etc/UTC"
			importStep("oncallalert_schedule_rule.sr", joinedID("oncallalert_schedule_rule.sr", "schedule_id", "id")),
			importStep("oncallalert_user_override.o", nil),
			importStep("oncallalert_escalation_policy.ep", nil),
			importStep("oncallalert_service.svc", nil),
			importStep("oncallalert_service_label.l", joinedID("oncallalert_service_label.l", "service_id", "key")),
			importStep("oncallalert_integration_key.k", nil),
		},
	})
}

func TestAccSchedule_RecreatedWhenDeletedOutsideTerraform(t *testing.T) {
	name := "tfacc-" + acctest.RandString(6)
	cfg := fmt.Sprintf(`
resource "oncallalert_schedule" "s" {
  name      = "%s-sched"
  time_zone = "UTC"
}
`, name)
	var firstID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: accFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					firstID = s.RootModule().Resources["oncallalert_schedule.s"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					if err := accClient(t).DeleteAll(context.Background(), "schedule", firstID); err != nil {
						t.Fatal(err)
					}
				},
				Config: cfg,
				Check: func(s *terraform.State) error {
					if id := s.RootModule().Resources["oncallalert_schedule.s"].Primary.ID; id == firstID {
						return fmt.Errorf("schedule was not recreated (id %s unchanged)", id)
					}
					return nil
				},
			},
		},
	})
}
