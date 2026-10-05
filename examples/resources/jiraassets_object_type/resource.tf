data "jiraassets_object_schema" "inventory" {
  id = "100"
}

# An abstract parent holding attributes its children inherit.
resource "jiraassets_object_type" "resource" {
  name             = "Resource"
  description      = "Everything in the inventory, whatever kind."
  icon_id          = "143"
  object_schema_id = data.jiraassets_object_schema.inventory.id
  abstract         = true
  inherited        = true

  attributes = {
    "Owner"        = "text"
    "Last Seen At" = "datetime"
  }
}

resource "jiraassets_object_type" "server" {
  name                  = "Server"
  description           = "A physical or virtual server"
  icon_id               = "143"
  object_schema_id      = data.jiraassets_object_schema.inventory.id
  parent_object_type_id = jiraassets_object_type.resource.id

  # Owner and Last Seen At are inherited from the parent.
  attributes = {
    "Hostname"       = "text"
    "CPU Cores"      = "integer"
    "Decommissioned" = "boolean"
  }
}

output "hostname_attribute_id" {
  value = jiraassets_object_type.server.attribute_ids["Hostname"]
}
