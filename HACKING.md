# Developing the provider

| Task | Command |
| --- | --- |
| Build | `go install` |
| Regenerate `docs/` | `go generate ./...` (needs `tofu` on `PATH`) |
| Regenerate `docs/` on commit | `prek install` or `pre-commit install` |
| Unit tests | `go test ./...` |
| Acceptance tests | `make testacc` |

Acceptance tests create real objects. They read credentials from the provider environment
variables, optionally via an untracked `.env`, and `JIRAASSETS_TEST_SCHEMA_ID` names a scratch
schema. They run under `tofu`; the Makefile sets `TF_ACC_TERRAFORM_PATH`, `TF_ACC_PROVIDER_HOST` and
`TF_ACC_PROVIDER_NAMESPACE`.
