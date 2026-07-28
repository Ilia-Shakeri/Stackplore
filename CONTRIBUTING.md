# Contributing to Stackplore

Thank you for helping improve Stackplore. The project is being delivered in numbered phases, and focused changes are easier to review, verify, and maintain.

## Before you begin

1. Read [PROJECT_GUIDE.md](PROJECT_GUIDE.md), [SPEC.md](SPEC.md), and [PLAN.md](PLAN.md).
2. Check existing issues and pull requests for related work.
3. Confirm which phase is active and keep the proposal within that phase.
4. Open an issue before substantial work so scope and design can be agreed upon.
5. Use the private process in [SECURITY.md](SECURITY.md) for vulnerabilities.

The repository currently contains the Phase 1 web and API developer scaffold,
builds, and basic tests. Product workflows remain planned. Follow
[docs/development.md](docs/development.md) for the pinned bootstrap and
verification commands, and do not imply that later-phase behavior exists.

## Scope and design

- Keep each pull request focused on one coherent concern.
- Do not include work assigned to a later phase.
- Preserve the documented security, provider, and portability boundaries.
- Prefer explicit, testable behavior over hidden conventions.
- Update relevant documentation when behavior or public contracts change.
- Record a meaningful architecture decision when a change alters a durable system boundary.
- Avoid unrelated formatting, renaming, or dependency changes.

If the proposed work conflicts with the specification, raise the conflict before implementation. A deliberate specification change should be documented rather than introduced indirectly.

## Security and privacy

Never submit:

- Passwords, access tokens, session values, or private keys
- Raw kubeconfig files or provider credentials
- Secret values from containers or clusters
- Private hostnames, addresses, account data, or unredacted production logs
- Sensitive data in screenshots, fixtures, examples, commits, or test output

Use clearly fictitious examples. Redact diagnostic output before attaching it, and describe the redaction when it affects reproduction.

## Making a change

1. Create a branch from the current default branch.
2. Make the smallest complete change that addresses the agreed scope.
3. Add or update tests when the relevant test foundation exists.
4. Run every check relevant to the changed area.
5. Review the complete diff for accidental files and sensitive data.
6. Update the changelog when the change is notable to users or operators.
7. Open a pull request using the repository template.

Do not mix generated build output, local configuration, editor state, or runtime data into a pull request.

## Code and documentation quality

- Use clear names and small, well-bounded modules.
- Keep security decisions visible at the boundary where they are enforced.
- Return actionable errors without leaking internal or sensitive data.
- Write comments for intent and constraints, not a restatement of the code.
- Keep English source comments and technical documentation concise and direct.
- Keep English and Persian user-facing content equivalent in meaning.
- Verify internal links and examples in documentation changes.

Language-specific formatting and lint rules will be documented when the application foundations are added. Follow the checked-in configuration and exact commands in `PROJECT_GUIDE.md` once they exist.

## Commits

Use Conventional Commits with a concise imperative subject:

```text
docs: clarify the provider security boundary
fix(api): reject an invalid resource filter
test(storage): cover migration rollback behavior
```

Keep commits reviewable and do not claim checks or runtime behavior that you did not verify.

## Pull requests

A pull request should state:

- The active phase and the issue or need being addressed
- What changed and what was intentionally left out
- Security, privacy, compatibility, and localization effects
- Exact verification commands and observed results
- Any limitation or environment-dependent check that remains

Screenshots are useful only for real visual changes. Remove all private data before attaching them.

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
