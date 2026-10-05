data "jiraassets_object_schema" "inventory" {
  id = "100"
}

resource "jiraassets_object_type" "server" {
  name             = "Server"
  icon_id          = "143"
  object_schema_id = data.jiraassets_object_schema.inventory.id
}

resource "jiraassets_object_type" "team" {
  name             = "Team"
  icon_id          = "143"
  object_schema_id = data.jiraassets_object_schema.inventory.id
  inherited        = true
}

resource "jiraassets_object_type" "squad" {
  name                  = "Squad"
  icon_id               = "143"
  object_schema_id      = data.jiraassets_object_schema.inventory.id
  parent_object_type_id = jiraassets_object_type.team.id
}

# type defaults to "default", which stores a value of data_type.
resource "jiraassets_object_type_attribute" "hostname" {
  object_type_id = jiraassets_object_type.server.id
  name           = "Hostname"
  data_type      = "text"
}

resource "jiraassets_object_type_attribute" "last_seen" {
  object_type_id = jiraassets_object_type.server.id
  name           = "Last Seen"
  data_type      = "datetime"
}

# A fixed set of values.
resource "jiraassets_object_type_attribute" "criticality" {
  object_type_id = jiraassets_object_type.server.id
  name           = "Criticality"
  data_type      = "select"
  options        = ["Low", "Medium", "High"]
}

# An unbounded list of integers.
resource "jiraassets_object_type_attribute" "open_ports" {
  object_type_id      = jiraassets_object_type.server.id
  name                = "Open Ports"
  data_type           = "integer"
  maximum_cardinality = -1
}

# Setting label adopts and renames the built-in Name attribute.
resource "jiraassets_object_type_attribute" "asset_tag" {
  object_type_id   = jiraassets_object_type.server.id
  name             = "Asset Tag"
  data_type        = "text"
  label            = true
  unique_attribute = true
}

resource "jiraassets_reference_type" "owned_by" {
  name             = "Owned By"
  color            = "42526E"
  object_schema_id = data.jiraassets_object_schema.inventory.id
}

# type_value is the target object type. include_child_object_types also
# accepts its children, here a Squad as well as a Team.
resource "jiraassets_object_type_attribute" "owner" {
  object_type_id             = jiraassets_object_type.server.id
  name                       = "Owner"
  type                       = "object_reference"
  type_value                 = jiraassets_object_type.team.id
  reference_type_id          = jiraassets_reference_type.owned_by.id
  include_child_object_types = true
}

resource "jiraassets_reference_type" "depends_on" {
  name             = "Depends On"
  color            = "42526E"
  object_schema_id = data.jiraassets_object_schema.inventory.id
}

# Many-to-many.
resource "jiraassets_object_type_attribute" "depends_on" {
  object_type_id      = jiraassets_object_type.server.id
  name                = "Depends On"
  type                = "object_reference"
  type_value          = jiraassets_object_type.server.id
  reference_type_id   = jiraassets_reference_type.depends_on.id
  maximum_cardinality = -1
}

# data_type applies only to type "default".
resource "jiraassets_object_type_attribute" "on_call" {
  object_type_id = jiraassets_object_type.server.id
  name           = "On Call"
  type           = "user"
}
