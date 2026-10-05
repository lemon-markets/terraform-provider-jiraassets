# Terraform Provider for Jira Assets

Manages [Jira Assets](https://support.atlassian.com/jira-service-management-cloud/docs/what-is-assets-in-jira-service-management/)
object types, attributes, reference types and objects. Development notes are in
[HACKING.md](./HACKING.md).

## Requirements

- [OpenTofu](https://opentofu.org) >= 1.6, or [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 to build

## Configuration

Authentication is basic auth with a Jira account email and an
[API token](https://id.atlassian.com/manage-profile/security/api-tokens).

```terraform
provider "jiraassets" {
  workspace_id = "00000000-0000-0000-0000-000000000000"
}
```

| Setting        | Environment variable      |
| -------------- | ------------------------- |
| `workspace_id` | `JIRAASSETS_WORKSPACE_ID` |
| `user`         | `JIRAASSETS_USER`         |
| `password`     | `JIRAASSETS_PASSWORD`     |

Get the workspace id with:

```shell
curl -su "$JIRAASSETS_USER:$JIRAASSETS_PASSWORD" \
  "https://your-site.atlassian.net/rest/servicedeskapi/assets/workspace"
```

## Resources and data sources

| Name                               | Type        | Purpose                                    |
| ---------------------------------- | ----------- | ------------------------------------------ |
| `jiraassets_object_type`           | Resource    | An object type and its plain attributes    |
| `jiraassets_object_type_attribute` | Resource    | One attribute on an object type            |
| `jiraassets_reference_type`        | Resource    | The relationship a reference attribute names |
| `jiraassets_object`                | Resource    | An object                                  |
| `jiraassets_object_schema`         | Data source | An existing object schema                  |

Reference docs are in [`docs/`](./docs).

## Behaviour

- A reference type's `object_schema_id` forces replacement.
- Changing an attribute's `data_type` does not convert existing values.
- Every object type has the built-in attributes `Key`, `Created`, `Updated` and `Name`. Declaring
  one by name, or declaring `label = true`, adopts the existing attribute. `Key`, `Created` and
  `Updated` are read-only. Destroying an adopted attribute removes it from state only.
- `inherited = true` passes an object type's attributes to all its descendants. It must be set at
  creation, on the topmost object type only, and changing it forces replacement.
- A child of an inherited object type owns no attributes, cannot reuse an inherited name with a
  different data type, and cannot hold the label.
- `abstract = true` prevents objects in an object type.
- `include_child_object_types` on a reference accepts objects of the target's children.
- Removing a name from an object type's `attributes` deletes that attribute and its values.
- An object type can have at most two unique attributes, and a unique attribute must have
  `maximum_cardinality = 1`.
- If an inline attribute fails during create, the object type stays in state but is tainted.
  Untaint it and apply again; replacing it would delete its objects.

## Inline attributes

```hcl
resource "jiraassets_object_type" "vpc" {
  name             = "AWS VPC"
  icon_id          = "143"
  object_schema_id = data.jiraassets_object_schema.inventory.id

  attributes = {
    "CIDR Block" = "text"
    "Is Default" = "boolean"
  }
}
```

`attribute_ids` maps each name in `attributes` to its id. `all_attribute_ids` maps every attribute
on the object type, inherited ones included. Attributes needing any other setting use
`jiraassets_object_type_attribute`. Do not declare one name both ways.
