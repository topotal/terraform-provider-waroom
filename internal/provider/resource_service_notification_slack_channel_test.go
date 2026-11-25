package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

// The test overwrites and removes the notification channel of two services, so it uses services created by
// acceptance tests (named "tf-acc-*") rather than a real one such as E2E_WAROOM_SERVICE_NAME.
//
// Waroom resolves the channel through the Slack API, so the channels must exist in the Slack workspace
// connected to the Waroom under test: E2E_WAROOM_SLACK_CHANNEL_ID is required, and E2E_WAROOM_SLACK_CHANNEL_ID_2
// (a different channel) enables the step that changes the channel in place.
func TestAccServiceNotificationSlackChannelResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	channelID := os.Getenv("E2E_WAROOM_SLACK_CHANNEL_ID")
	if channelID == "" {
		t.Skip("E2E_WAROOM_SLACK_CHANNEL_ID must be set to the ID of a channel in the Slack workspace connected to Waroom")
	}
	testAccPreCheck(t)

	services := testAccServices(t, 2)
	serviceA, serviceB := services[0], services[1]
	finalChannelID := channelID

	const name = "waroom_service_notification_slack_channel.test"

	steps := []resource.TestStep{
		// A malformed channel ID is rejected at plan time.
		// This step comes first because the post-test destroy uses the last step's config.
		{
			Config:      testAccServiceNotificationSlackChannelResourceConfig(serviceA, "#general"),
			ExpectError: regexp.MustCompile(`must be a Slack channel ID`),
		},
		// Create and Read
		{
			Config: testAccServiceNotificationSlackChannelResourceConfig(serviceA, channelID),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(serviceA)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceA)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New("channel_id"), knownvalue.StringExact(channelID)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New("channel_name"), knownvalue.StringRegexp(regexp.MustCompile(`.+`))),
			},
		},
		// ImportState
		{
			ResourceName:      name,
			ImportState:       true,
			ImportStateVerify: true,
		},
	}

	if channelID2 := os.Getenv("E2E_WAROOM_SLACK_CHANNEL_ID_2"); channelID2 != "" {
		finalChannelID = channelID2
		// Update channel_id in place
		steps = append(steps, resource.TestStep{
			Config: testAccServiceNotificationSlackChannelResourceConfig(serviceA, channelID2),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
					plancheck.ExpectUnknownValue(name, tfjsonpath.New("channel_name")),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(name, tfjsonpath.New("channel_id"), knownvalue.StringExact(channelID2)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New("channel_name"), knownvalue.StringRegexp(regexp.MustCompile(`.+`))),
			},
		})
	} else {
		t.Log("E2E_WAROOM_SLACK_CHANNEL_ID_2 is not set; skipping the in-place update step")
	}

	steps = append(steps,
		// A channel removed outside Terraform is treated as absent and set again
		resource.TestStep{
			PreConfig: func() {
				err := testAccWaroomClient(t).Services.DeleteNotificationSlackChannel(context.Background(),
					waroom.DeleteServiceNotificationSlackChannelParams{Name: serviceA})
				if err != nil {
					t.Fatalf("delete notification slack channel of %s: %s", serviceA, err)
				}
			},
			Config: testAccServiceNotificationSlackChannelResourceConfig(serviceA, finalChannelID),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionCreate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(name, tfjsonpath.New("channel_id"), knownvalue.StringExact(finalChannelID)),
			},
		},
		// Changing the service replaces the resource
		resource.TestStep{
			Config: testAccServiceNotificationSlackChannelResourceConfig(serviceB, finalChannelID),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(serviceB)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceB)),
			},
		},
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceNotificationSlackChannelDestroy(t),
		Steps:                    steps,
	})
}

// testAccCheckServiceNotificationSlackChannelDestroy expects no channel to be set on the service (404).
func testAccCheckServiceNotificationSlackChannelDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_service_notification_slack_channel" {
				continue
			}
			serviceName := rs.Primary.Attributes["service_name"]
			_, err := client.Services.GetNotificationSlackChannel(context.Background(),
				waroom.GetServiceNotificationSlackChannelParams{Name: serviceName})
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				continue
			}
			if err != nil {
				return fmt.Errorf("get notification slack channel of %s: %w", serviceName, err)
			}
			return fmt.Errorf("notification slack channel of %s still exists", serviceName)
		}
		return nil
	}
}

func testAccServiceNotificationSlackChannelResourceConfig(serviceName, channelID string) string {
	return fmt.Sprintf(`
resource "waroom_service_notification_slack_channel" "test" {
  service_name = %q
  channel_id   = %q
}
`, serviceName, channelID)
}
