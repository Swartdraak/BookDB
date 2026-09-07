# Repository Operations

This document collects the repository-level setup commands for a clean clone.

## Dev container

Open the repository in a Dev Container. The container includes Go 1.27, Docker CLI access, and the editor extensions needed for root tooling.

## Root commands

```bash
make setup
make fmt
make fmt-check
make lint
make test
make build
make openapi-validate
make compose-up
make compose-down
make integration
```

## Command intent

- `make setup` downloads Go modules for the tooling and validator.
- `make fmt` rewrites Go files with `gofmt`.
- `make fmt-check` fails if any Go file is not formatted.
- `make lint` runs `go vet ./...`.
- `make test` runs the full Go test suite.
- `make build` compiles all Go packages.
- `make openapi-validate` checks `api/openapi-outline.yaml` for a valid OpenAPI 3.1 outline.
- `make compose-up` starts the development Compose stack.
- `make compose-down` stops the development Compose stack and removes volumes.
- `make integration` runs the compose smoke test and guarantees cleanup on exit.

## Branch protection expectations

`main` should remain protected with:

- required pull requests;
- required status checks for `fmt-check`, `lint`, `test`, `build`, `openapi-validate`, compose smoke, dependency review, secret scan, SBOM generation, and image vulnerability scan;
- CODEOWNERS review where an ownership file is present;
- conversation resolution before merge.

The detailed policy overview is in [docs/27_GITHUB_REPOSITORY_BLUEPRINT.md](docs/27_GITHUB_REPOSITORY_BLUEPRINT.md).