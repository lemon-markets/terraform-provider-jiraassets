# Terraform Provider for Jira Assets

Manages [Jira Assets](https://support.atlassian.com/jira-service-management-cloud/docs/what-is-assets-in-jira-service-management/)
object schemas, object types, attributes, reference types and objects as Terraform resources.
Built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Requirements

- [OpenTofu](https://opentofu.org) >= 1.6, or [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (to build the provider)

## Using the provider

The provider authenticates with basic auth against the Assets REST API: a Jira account email and
an [API token](https://id.atlassian.com/manage-profile/security/api-tokens). All three settings
fall back to environment variables, which is the better option for the token.

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

The workspace id is not shown in the Jira UI. Retrieve it with:

```shell
curl -su "$JIRAASSETS_USER:$JIRAASSETS_PASSWORD" \
  "https://your-site.atlassian.net/rest/servicedeskapi/assets/workspace"
```

### Resources and data sources

| Name                                     | Type        | Purpose                                         |
| ---------------------------------------- | ----------- | ----------------------------------------------- |
| `jiraassets_object_schema`               | Resource    | An object schema, the top-level container       |
| `jiraassets_object_type`                 | Resource    | An object type within a schema                  |
| `jiraassets_object_type_attribute`       | Resource    | An attribute on an object type                   |
| `jiraassets_reference_type`              | Resource    | Labels what an object reference attribute means |
| `jiraassets_object`                      | Resource    | An individual object                            |
| `jiraassets_object_schema`               | Data source | Looks up an existing schema by id               |
| `jiraassets_object_type`                 | Data source | Looks up an existing object type by id          |

Per-resource reference, including import syntax, is under [`docs/`](./docs).

### A note on updates

The Assets API accepts some writes it then silently ignores. Where that happens the schema
documents it and forces replacement, rather than reporting a successful apply that changed
nothing:

- `object_schema_key`, and a reference type's `object_schema_id`, return 200 on update with the
  old value still in the response body.
- Retyping an attribute (`data_type`) succeeds even when existing values are incompatible with the
  new type, with no validation or migration. A clean plan does not mean safe data.
- Creating an attribute with `label = true` steals the label from whichever attribute held it
  before, including the built-in `Name`, instead of erroring. See below.
- An object type's `PUT` rejects its own current name with "Name has to be unique within the same
  object schema": the uniqueness check counts the object type itself. The provider sends the name
  only when it changes, otherwise no attribute-only or description-only update could ever apply.

### Built-in attributes

Every object type is born with four attributes the API creates itself: `Key`, `Created`, `Updated`
and `Name`. None can be deleted, and `Name` holds the object type's label.

Declaring an attribute that collides with one of those adopts it instead of creating a second
attribute — by name, or by `label = true` for whichever attribute currently holds the label. So
declaring `Name` is enough to manage the label and give it a description; no `import` block is
needed, and nothing is duplicated.

The label is the object's name everywhere Jira shows one: the schema tree, reference pickers, the
Assets field on a ticket, AQL results. Keep it readable and put uniqueness on the identifier
instead — a label of `arn:aws:rds:eu-central-1:…` is unusable in a picker, and it is what a
reference to that object displays as too.

Only these four are ever adopted (they are the only attributes with `removable: false`), so
adoption cannot capture an attribute another resource manages. `Key`, `Created` and `Updated` are
not editable, and declaring them is an error rather than a failed apply. Destroying an adopted
attribute drops it from state and leaves it in place, since the API refuses to delete it.

### Nested object types

An object type has at most one parent and any number of children, and three flags govern what that
nesting does:

| Attribute                                          | Effect                                                             |
| -------------------------------------------------- | ------------------------------------------------------------------ |
| `inherited` on the parent                          | Passes the parent's attributes down to every child                 |
| `abstract_object_type`                             | Stops the object type holding objects, leaving it a holder of attributes |
| `include_child_object_types` on a reference        | Accepts objects of the target's children, not only the target      |

`inherited` **must be set when the object type is created**. The API refuses to enable it on an
object type that already has children, and its payload cannot express turning it off, so the
provider forces replacement in either direction rather than reporting a change it cannot make.
Replacing a parent cascades to its children — plan a hierarchy before applying it.

Set it on the topmost object type only and leave it unset below. Inheritance propagates: a child of
an inherited object type comes back `inherited = true` whether it asked or not, so an explicit
`inherited = false` on a child is a conflict the provider reports rather than passing off as drift.

An abstract parent with `inherited = true` is how you share columns across object types: the
attributes exist once and every child sees them, rather than each object type carrying its own copy,
and AQL can query the parent to reach every child.

What inheritance costs, all of it measured against the API rather than inferred:

- **A child owns nothing.** Not one attribute, not even the built-in `Key`, `Created`, `Updated` and
  `Name` — they stay with the parent and carry the parent's ids. `excludeParentAttributes=true` on a
  child returns an empty list.
- **An inherited name is taken.** Creating an attribute the parent already supplies fails with "Name
  has to be unique!". Where a child's `attributes` names one anyway, the provider records the
  parent's id instead of creating a duplicate — which is also the id anything writing a value needs.
  If the inherited attribute has a different data type, that is an error: retyping it would change
  every object type that inherits it.
- **A child cannot hold the label.** "Impossible to set child's attribute as label if inheritance."
  Declare the label on the object type that owns the attributes; every child shows it. So a
  hierarchy gets one label for the whole subtree, not one per child.

`parent_object_type_inherited` is read-only. It reports whether the object type was given a copy of
its parent's attributes at creation — the UI's "Add parent attributes" — which the object type
payload has no field for.

### Inline attributes

An object type's plain columns are a map of name to data type, so the common case needs no separate
resource per attribute:

```hcl
resource "jiraassets_object_type" "vpc" {
  name             = "AWS VPC"
  icon_id          = "143"
  object_schema_id = jiraassets_object_schema.inventory.id

  attributes = {
    "CIDR Block" = "text"
    "Is Default" = "boolean"
  }
}
```

`attribute_ids` maps each of those names to its attribute id. Where several object types share
columns, put them on a parent with `inherited = true` rather than repeating the map — see above.

The map holds plain name-to-data-type entries only. An attribute that needs a label, a reference,
uniqueness, cardinality, a suffix or a description stays a `jiraassets_object_type_attribute`
resource — they are not either/or, and most object types use both. What they must not do is name the
same attribute twice: the API accepts two attributes with one name, and the second would shadow the
first.

An object type takes over an existing attribute of the same name rather than duplicating it, so the
map can be layered onto an imported object type. The corollary is that removing a name from it
deletes that attribute and every value in it. There is no separate confirmation for that the way
there is for a resource being destroyed, only the plan.

If an object type is created but one of its attributes fails, the object type is still written to
state so it does not leak. Terraform taints a resource whose create returned an error, though, and a
replacement here would delete the object type and its objects — so untaint it and apply again rather
than accepting that plan. The sync takes over whatever already exists, which makes the retry safe.

## Developing the provider

To compile the provider, run `go install`. To run it against a local configuration without
publishing it, use a
[development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).

To regenerate documentation from the schema and the files in `examples/`, run:

```shell
go generate ./...
```

This needs `tofu` on `PATH`; no `terraform` binary is required anywhere in this repo.
`tfplugindocs` would otherwise export the schema by building the provider and invoking a CLI
itself, but only ever `terraform` — it hardcodes `registry.terraform.io` in the plugin mirror path
it writes and in the schema key it reads back, and downloads a binary when none is found, which
currently fails against an expired HashiCorp signing key. So `scripts/generate.sh` exports the
schema with `tofu` and passes it in via `-providers-schema`, skipping both the build and the CLI
call. Set `TOFU` to use a specific binary.

To run the acceptance tests, run `make testacc`. They create real objects in the configured
workspace, so point them at a scratch schema.

```shell
make testacc
```

The Makefile points the test harness at `tofu` too. That needs three variables, which it sets for
you: `TF_ACC_TERRAFORM_PATH` (an absolute path — it is used as an exact binary path, not looked up
on `PATH`), plus `TF_ACC_PROVIDER_HOST` and `TF_ACC_PROVIDER_NAMESPACE`, because the harness builds
a provider reattach address under `registry.terraform.io` that tofu rejects as an invalid
namespace.

The `GNUmakefile` does `-include .env`, so credentials can be kept in an untracked `.env` file, and
`make` still works without one.

## Adding dependencies

This provider uses [Go modules](https://go.dev/wiki/Modules). To add a dependency:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.
