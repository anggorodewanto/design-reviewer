# Implement Phase {N}

Read and implement the spec at `docs/phase-{N}-*.md`. Follow these steps in order:

## Step 1: Understand Context

1. Read the phase spec file in `docs/` matching phase number `{N}`
2. Read `docs/SPEC.md` for overall project context
3. Read `docs/TESTING.md` for testing conventions
4. Explore the existing codebase to understand current state — check `internal/`, `cmd/`, `web/`, and `integration_test.go`

## Step 2: Implement

1. Implement everything described in the phase spec
2. Keep code minimal — no unnecessary abstractions or code that doesn't directly serve the spec
3. Follow patterns already established in the codebase
4. Run `go build ./...` to confirm compilation

## Step 3: Verify Against Spec

Go through every requirement in the phase spec and verify:

1. Every struct, method, endpoint, and behavior described in the spec exists and works correctly
2. Every route is registered and responds as documented
3. Every edge case mentioned in the spec is handled
4. Run the server and manually test with curl commands from the spec's Verification section if provided
5. List any discrepancies found and fix them before proceeding

## Step 4: Write Tests

### Unit Tests

- Create `*_test.go` files next to the packages you modified (e.g., `internal/storage/storage_test.go`)
- Test each exported function/method in isolation
- Cover happy paths, error cases, and edge cases
- Mock or use temp resources (temp dirs, in-memory DBs) — no test pollution

### Integration Tests

- Add new test functions to the existing `integration_test.go` at the project root
- Mark the new section with a comment: `// --- Phase {N}: <Title> ---`
- Use the existing helpers (`setup`, `makeZip`, `uploadZip`) and add new helpers as needed
- Test full HTTP request/response flows for every new endpoint
- Test interactions between this phase and previous phases

### Coverage Target

- Aim for the best possible coverage on all new and modified packages — cover every reachable code path
- Run: `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`
- Review uncovered lines and add tests for any remaining gaps

### All Tests Must Pass

- Run `go test -count=1 ./...` after writing tests — **every test must pass**, both new and pre-existing
- If any pre-existing test fails, investigate and fix it before proceeding — the failure may be caused by your changes or by a missing element the test expects
- Do not move on to the next step until the full suite is green
- Show the final coverage output

## Step 5: Final Check

1. Run `go test -v -count=1 ./...` — **all tests must pass** (new AND existing), zero failures
2. If any test fails, fix the issue and re-run until the entire suite is green
3. Run `go build ./...` — must compile cleanly
4. Show a summary of: files created/modified, test count, coverage per package
