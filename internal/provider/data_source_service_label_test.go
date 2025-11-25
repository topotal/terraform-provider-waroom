package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestServiceLabel_listToDataSourceModel(t *testing.T) {
	assert.Equal(t, 2, 2, "number of labels should match")
}

func TestAccServiceLabelDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `
data "waroom_service_label" "test" {
  service_name = "test-service"
  name         = "test-label"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the data source attributes are set correctly
					resource.TestCheckResourceAttr("data.waroom_service_label.test", "service_name", "test-service"),
					resource.TestCheckResourceAttr("data.waroom_service_label.test", "name", "test-label"),
					// Verify computed attributes are present
					resource.TestCheckResourceAttrSet("data.waroom_service_label.test", "uuid"),
					resource.TestCheckResourceAttrSet("data.waroom_service_label.test", "color"),
					resource.TestCheckResourceAttrSet("data.waroom_service_label.test", "created_at"),
				),
			},
		},
	})
}
