package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccObjectTypeAttributeDataTypes checks that the API reads time, url, email
// and select back as the codes this provider sends for them.
func TestAccObjectTypeAttributeDataTypes(t *testing.T) {
	schemaId := os.Getenv("JIRAASSETS_TEST_SCHEMA_ID")
	if schemaId == "" {
		t.Skip("JIRAASSETS_TEST_SCHEMA_ID must name a scratch object schema")
	}
	iconId := os.Getenv("JIRAASSETS_TEST_ICON_ID")
	if iconId == "" {
		iconId = "143"
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "jiraassets_object_type" "test" {
  name             = "tf-acc data types"
  icon_id          = %[2]q
  object_schema_id = %[1]q
}

resource "jiraassets_object_type_attribute" "time" {
  object_type_id = jiraassets_object_type.test.id
  name           = "Maintenance Window"
  data_type      = "time"
}

resource "jiraassets_object_type_attribute" "url" {
  object_type_id = jiraassets_object_type.test.id
  name           = "Homepage"
  data_type      = "url"
}

resource "jiraassets_object_type_attribute" "email" {
  object_type_id = jiraassets_object_type.test.id
  name           = "Owner Email"
  data_type      = "email"
}

resource "jiraassets_object_type_attribute" "criticality" {
  object_type_id = jiraassets_object_type.test.id
  name           = "Criticality"
  data_type      = "select"
  options        = ["Low", "Medium", "High"]
}
`, schemaId, iconId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("jiraassets_object_type_attribute.time", "data_type", "time"),
					resource.TestCheckResourceAttr("jiraassets_object_type_attribute.url", "data_type", "url"),
					resource.TestCheckResourceAttr("jiraassets_object_type_attribute.email", "data_type", "email"),
					resource.TestCheckResourceAttr("jiraassets_object_type_attribute.criticality", "options.#", "3"),
					resource.TestCheckResourceAttr("jiraassets_object_type_attribute.criticality", "options.2", "High"),
				),
			},
			{
				// The refresh after the attributes exist lists them on the object type.
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("jiraassets_object_type.test", "all_attribute_ids.Homepage", "jiraassets_object_type_attribute.url", "id"),
					resource.TestCheckResourceAttrSet("jiraassets_object_type.test", "all_attribute_ids.Name"),
				),
			},
		},
	})
}
