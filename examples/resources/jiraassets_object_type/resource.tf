data "jiraassets_object_schema" "inventory" {
  id = "100"
}

# Attributes shared by several object types go on an abstract parent, which holds
# them for its children and cannot contain objects of its own. inherited passes
# them down, and has to be set here at creation: the API refuses to enable it once
# the object type has children.
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

  # Only this object type's own columns. Owner and Last Seen At arrive by
  # inheritance and stay owned by the parent, ids included, so they are not
  # repeated here.
  attributes = {
    "Hostname"       = "text"
    "CPU Cores"      = "integer"
    "Decommissioned" = "boolean"
  }
}

output "hostname_attribute_id" {
  value = jiraassets_object_type.server.attribute_ids["Hostname"]
}
