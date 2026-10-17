# Provider and isolated demonstration checkpoint — 2026-10-17

This evidence covers disposable local targets. It does not establish production
installation, capacity, complete assistive-technology acceptance or final 1.0
delivery. Stages 6–9 remain open in the [active plan](../../implementation-plan.md).

- [Guest runtime](guest-runtime.json): real public reads without authentication
  cookies, seven private/write HTTP rejections, zero accounts, sessions, recent
  grants or outbound deliveries. Restart advanced the protected reset epoch from
  1 to 2 while restoring nine resources and 972 synthetic metric facts; prior
  protected audit history survived.
- [Loaded browser matrix](browser-matrix.json): six routes, both locales and two
  viewport/theme profiles (360 light, 1280 dark), 24 successful loaded states.
  Direction, language and horizontal overflow were checked against actual data.
  Scrollbar width is excluded from the document client width.
- [Persian mobile overview](guest-overview-fa-mobile.png),
  [English dark topology](guest-topology-en.png), and
  [Persian light topology](guest-topology-fa.png) show loaded fixture data. The
  Persian light graph is an additional visual check outside the 24-state matrix.
- [Verification receipts](verification.json) record commands, bounded outcomes,
  tool versions and hashes of private local logs, without credentials or keys.
- [SHA-256 checksums](SHA256SUMS) identify the committed evidence files.

The actual nine-node/two-environment journey showed health counts 7/1/1, resource
detail and log links, and a visible read-only guest boundary. These browser checks
use semantic DOM and visual inspection; no native screen-reader process was run.
Initial cached/loading screenshots were replaced with direct loaded captures.

The Docker proof used a real Engine 29.1.3, negotiated API 1.52 and the restricted
mTLS proxy: inventory, CPU/memory/network, safe metadata, both log framing modes,
events, client identity and unsafe-method/path denials passed. Its owned
containers, network, volume and temporary images were removed. Early cold-build,
certificate ownership and overly broad test-string failures remain separate
failed receipts; they are not successful provider runs.

The Kubernetes proof used a disposable kind 0.33.0 / Kubernetes 1.37.0 target and
the same final test binary for both missing Metrics API (7.06 seconds) and present
Metrics API (18.84 seconds). Namespace permissions, partial kinds, Pod UID/log
identity, usage identity/lifetime, denied metrics and permitted single-namespace
usage passed. Metrics Server 0.9.0 was installed from verified official artifacts.
The owned cluster and its private kubeconfig were removed. Earlier registry and
local fixture setup failures remain failed receipts.

The native PostgreSQL 17.11 drill passed fixture seeding, alert acknowledgement,
session revocation, source versioning, event-stream replay across restart,
PostgreSQL-native backup/restore and loss-of-ownership shutdown. It verified 12
migrations, nine resources, 972 metric facts, one seeded alert and no outbound
delivery. This drill predates final read-side environment/metric-clock fixes;
the final full PostgreSQL gate separately covers those changes.

The first broad local gate exposed a one-second test-fixture deadline under host
load. Its test-only deadline was bounded at ten seconds and full fixture counts
were strengthened to 972 metrics and nine events. The next PostgreSQL gate
exposed two metric-clock regressions; both were fixed without weakening their
assertions. A separate regression reproduced and closed omitted-environment name
disclosure before pagination. Final startup review also reproduced and fixed a
source-settings allowlist omission for the restricted Docker profile; its
negative and successful regression receipts remain separate. Final gate status belongs to the verification
receipt and active plan. Raw private receipts stay outside the repository.
