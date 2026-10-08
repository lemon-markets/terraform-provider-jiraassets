package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccJiraAssetsObjectResource(t *testing.T) {
	if os.Getenv("JIRAASSETS_WORKSPACE_ID") == "" {
		t.Skip("JIRAASSETS_WORKSPACE_ID and credentials must be set")
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "jiraassets_object" "test" {
					type_id = "117"
					attributes = [
						{
							attr_type_id = "1087"
							attr_value = "My Phone"
						},
						{
							attr_type_id = "1090"
							attr_value = "1234657890"
						}
					]
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("jiraassets_object.test", "attributes.#", "2"),
				),
			},
			{
				Config: `resource "jiraassets_object" "test_avatar" {
					type_id = "117"
					attributes = [
						{
							attr_type_id = "1087"
							attr_value = "My Phone"
						},
						{
							attr_type_id = "1090"
							attr_value = "1234657890"
						}
					]
					has_avatar = true
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("jiraassets_object.test_avatar", "attributes.#", "2"),
				),
			},
		},
	})
}
