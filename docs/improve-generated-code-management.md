# Plan: Improve Generated Code Management

## Problem

The project's protobuf code generation has several gaps that create friction and risk:

1. **No version pinning for `protoc-gen-go`** — The `Makefile` (`setup` target) and CI (`.github/workflows/test.yml`) both install `protoc-gen-go@latest`. Different developers (or CI runs at different times) can produce different generated output from the same `.proto` file.
2. **No proto linting or breaking change detection** — There's nothing preventing accidental breaking changes to `api/v1/events/transit.proto` (renamed fields, changed numbers, removed messages).
3. **Manually vendored third-party protos** — `third_party/gtfs/gtfs-realtime.proto` is checked in with no record of which upstream version it corresponds to or how to update it.
4. **Unused `generate-openapi` target** — Dead code in the Makefile with no CI integration.

## Proposed Changes

### Phase 1: Pin `protoc-gen-go` version

**Goal:** Deterministic code generation across all environments.

**Changes:**

- In `Makefile`, change the `setup` target:
  ```makefile
  setup:
  	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.3
  	go mod download
  	@echo "Setup complete. Ensure 'protoc' and 'golangci-lint' are installed on your system."
  ```
  The pinned version (`v1.36.3`) matches the `google.golang.org/protobuf` runtime version in `go.mod`.

- In `.github/workflows/test.yml`, pin the same version in both `lint` and `test` jobs:
  ```yaml
  - name: Install Protoc
    run: |
      sudo apt-get update
      sudo apt-get install -y protobuf-compiler
      go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.3
  ```

**Validation:** Run `make generate && make test` before and after — the generated `transit.pb.go` should be byte-identical if already using `v1.36.3`.

### Phase 2: Adopt `buf` for proto management

**Goal:** Linting, breaking change detection, and managed dependencies for proto files.

**Changes:**

- Add `buf.yaml` at the repo root:
  ```yaml
  version: v2
  modules:
    - path: api/v1/events
  deps:
    - buf.build/googl/protobuf
  lint:
    use:
      - STANDARD
  breaking:
    use:
      - FILE
  ```

- Add `buf.gen.yaml` at the repo root:
  ```yaml
  version: v2
  plugins:
    - remote: buf.build/protocolbuffers/go
      out: .
      opt:
        - paths=source_relative
        - Mthird_party/gtfs/gtfs-realtime.proto=github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs
  ```

- Update `Makefile`:
  ```makefile
  generate:
  	buf generate

  lint-proto:
  	buf lint
  	buf breaking --against '.git#branch=main'
  ```

- Update CI to install `buf` instead of raw `protoc`:
  ```yaml
  - name: Install buf
    uses: bufbuild/buf-setup-action@v1
    with:
      version: '1.50.0'  # pin to specific version

  - name: Lint protos
    run: buf lint

  - name: Check breaking changes
    run: buf breaking --against 'https://github.com/fjlanasa/tpm-go.git#branch=main'

  - name: Generate
    run: buf generate
  ```

- Remove `third_party/gtfs/gtfs-realtime.proto` once the GTFS dependency is managed via `buf.yaml` `deps` (or keep it if `buf` BSR doesn't have the GTFS module — in that case, declare it as a local module path in `buf.yaml`).

**Validation:** `buf lint` passes, `buf generate` produces identical output to the old `protoc` command, `buf breaking` passes against `main`.

### Phase 3: Clean up dead code

**Goal:** Remove unused targets that create confusion.

**Changes:**

- Remove the `generate-openapi` target from the `Makefile` (line 12-13). If OpenAPI generation is needed in the future, it can be re-added with proper CI integration at that time.
- Remove the `.gitignore` entry for `api/v1/events/transit.events.json` (line 15) since nothing generates it.

**Validation:** `make test` still passes.

## Ordering and Dependencies

```
Phase 1 (pin versions)  →  can be done immediately, no new tooling
Phase 2 (adopt buf)     →  requires installing buf; can follow Phase 1
Phase 3 (cleanup)       →  independent, can be done anytime
```

Phase 1 is the highest-value, lowest-risk change. Phase 2 is the most impactful but requires the team to agree on adopting `buf` as a dependency.

## Alternative Considered: Commit Generated Code

Instead of improving the generation pipeline, an alternative is to check `*.pb.go` files into git (remove the `**.pb.go` line from `.gitignore`). This eliminates the `protoc` toolchain requirement entirely for contributors who don't touch protos.

**Tradeoffs:**
- Pro: `go install` / `go test` works with zero extra tooling
- Pro: Diffs show exactly what changed in generated code on proto updates
- Con: Noisy diffs on proto changes (generated file is ~1,000 lines)
- Con: Risk of generated code drifting from proto definitions if someone forgets to regenerate

This is a valid approach used by many Go projects (including `google.golang.org/protobuf` itself) but is a team preference decision.
