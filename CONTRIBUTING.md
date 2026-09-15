# Contributing

Workspace Ops is currently an early P0 Product. Keep changes small, observable, and explicit about compatibility impact.

## Commit messages

Use:

```text
<type>: <imperative summary>
```

Preferred types:

```text
feat:      Product behavior
fix:       Product defect repair
docs:      documentation only
test:      tests / fixtures
refactor:  no intended Product behavior change
ci:        CI / validation
chore:     repository maintenance
contract:  explicit Product-visible compatibility contract change
```

A semantic Reference change belongs in the Workspace Ops Reference repository, not hidden inside a Product implementation commit.

```text
implementation convenience
!= semantic authority
```

## P0 rules

- Keep Core behavior read-only.
- Do not add implicit network fetch/pull behavior to inspection commands.
- Do not import private Lab state, identifiers, paths, or operational artifacts.
- Preserve unavailable evidence as unknown rather than inventing certainty.
- Do not freeze incidental Bash details as stable external contracts without review.
- Add or update regression coverage for Product-visible behavior changes.

## Validation

```bash
bash -n bin/wsp
bash tests/selftest.sh
```
