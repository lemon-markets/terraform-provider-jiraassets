## 1.0.0-beta (Unreleased)

FEATURES:

* **New Resource:** `jiraassets_object_schema`
* **New Resource:** `jiraassets_object_type`
* **New Resource:** `jiraassets_object_type_attribute`
* **New Resource:** `jiraassets_reference_type`
* **New Data Source:** `jiraassets_object_type`
* `jiraassets_object_type` gains an `attributes` map of name to data type, so an
  object type's plain columns need no resource each, and a computed
  `attribute_ids` mapping those names to ids. Columns shared between object types
  belong on a parent with `inherited = true`; a provider-level
  `default_attributes` block was tried for that and dropped, since it gave every
  object type its own copy of each attribute.
* Nested object types: `inherited` on `jiraassets_object_type` passes its
  attributes to its children, and `include_child_object_types` on a reference
  attribute accepts objects of the target's children. `abstract_object_type`
  gained the documentation to go with them, and the read-only
  `parent_object_type_inherited` reports the UI's "Add parent attributes".
  Inline attributes reuse an inherited attribute rather than failing on "Name
  has to be unique!", and report an error rather than retyping one out from
  under its owner. The two constraints the API only reveals on apply — a child
  cannot hold the label, and inheritance propagates to every descendant — are
  now provider diagnostics that name the object type responsible.

IMPROVEMENTS:

* Docs and examples for every resource and data source, generated without a
  terraform binary; OpenTofu drives doc generation and the acceptance tests.
* `jiraassets_object_type_attribute` defaults its cardinality instead of
  requiring it to be set.
* An object type takes over an existing attribute of the same name rather than
  creating a duplicate, so inline attributes can be layered onto an imported
  object type.

BUG FIXES:

* `workspace_id` configuration errors now report against the right attribute
  path.
* Nil attribute values from the API no longer panic on read.
* Updating a `jiraassets_object_type` no longer fails with "Name has to be
  unique within the same object schema". The API's uniqueness check counts the
  object type itself, so the name is sent only when it changes; every
  description-only update was unappliable before.
* API errors now carry the response body. go-atlassian reduces a 400 to
  "client: atlassian invalid payload" and drops the part naming the field.
* A child object type no longer treats its inherited attributes as its own. It
  cannot retype, delete or adopt an attribute belonging to its parent, and the
  standalone attribute resource refuses to create one whose name a parent
  already supplies instead of failing on the API's uniqueness check.

## 0.1.0 (Unreleased)

FEATURES:
