## 1.0.0-beta (Unreleased)

FEATURES:

* **New Resource:** `jiraassets_object_type`
* **New Resource:** `jiraassets_object_type_attribute`
* **New Resource:** `jiraassets_reference_type`
* `jiraassets_object_type`: `attributes`, `attribute_ids`, `all_attribute_ids`,
  `inherited`, `abstract`, `parent_object_type_inherited`.
* `jiraassets_object_type_attribute`: `options` for `select`; `time`, `url` and
  `email` data types; `include_child_object_types`.
* Built-in attributes are adopted instead of duplicated.
* Plan-time validation of unique attributes, labels on inherited children, and
  settings that apply only to references or selects.

IMPROVEMENTS:

* go-atlassian v2.
* Docs generated and acceptance tests run with OpenTofu.

BUG FIXES:

* `workspace_id` errors report the correct attribute path.
* Nil attribute values no longer panic on read.
* Updating an object type without renaming it no longer fails on name
  uniqueness.
* API errors include the response body.

## 0.1.0 (Unreleased)

FEATURES:
