data "jiraassets_object_schema" "inventory" {
  id = "100"
}

# Scoped to one schema. Omit object_schema_id to make it global.
resource "jiraassets_reference_type" "depends_on_ref" {
  name             = "Depends On"
  description      = "The source object cannot run without the target"
  color            = "42526E"
  object_schema_id = data.jiraassets_object_schema.inventory.id
}
