# Web interaction motion

The command palette is frequent and keyboard-driven: both its dialog and
scrim appear instantly. Focus management, Escape, navigation and owner checks
stay with the existing dialog primitive. A focused regression checks that both
layers opt out of motion.

Occasional notifications use an opacity transition with the shared 180 ms
duration and strong ease-out token. Starting-style supplies a progressive
entrance; unsupported browsers display the notification immediately. Swipe
tracking stays direct and existing cancellation behavior is retained. No
spring or new animation dependency is introduced.

Enter and exit tokens use cubic-bezier(0.23, 1, 0.32, 1). Reduced-motion mode
removes dialog/drawer travel and keeps existing shortened color/opacity feedback.
Reduced-transparency mode removes scrim blur. Reading charts and dashboard
values do not animate for decoration.

All execution and interaction checks run on GitHub Actions. The browser matrix
now exercises three consecutive Escape/shortcut reopen cycles, restored trigger
focus, cleared search and final route focus in both locales and themes. Focused
notification tests cover immediate dismissal followed by a new notice, queue
limits and explicit actions. Fresh successful receipts, toast swipe,
reduced motion and reduced transparency remain required; source review is not
visual or device-performance acceptance. These changes do not finish stable 2.0.

The previous candidate fc75a76 passed 43 focused interface tests on hosted run
41615128733. Its full gate stopped at formatting in three files; the hosted
formatting patch was applied. This is a failed gate, not release acceptance.

Light, dark and system themes, login, initial administrator setup, password
changes, recovery and user administration already exist. Display preferences
include language, calendar, digits and hour cycle. Theme synchronization now resets
to the configured default when another tab clears browser storage. Preferences
remain browser-local; this does not add server-side cross-device settings.

The same clear-storage correction applies to locale and document direction.
The normal authenticated browser journey now exercises the actual theme select
on both setup and login, and dismisses the real user-created notification. These
checks run with English and Persian fixtures; new execution receipts are pending.

The user-action modal also records its invoking control before taking focus and
restores that control on close. If an operation removes or disables the control,
focus returns to the main region. Both a focused regression and the normal
browser journey cover three repeated create-dialog cancellations. This closes
a source-level gap where the modal had no registered trigger for focus return.

Hosted run 41617009582 passed the expanded focused checks and formatting, then
failed the full lint gate on eight existing dashboard/chart issues: six void
callback shorthands, one redundant optional chain and one implicit numeric
template expression. The source correction preserves behavior and all lint rules.
The failed receipt remains failed; the corrected source requires a fresh run.

Hosted run 41617812858 passed seven infrastructure/runtime jobs and 649 of 650
web tests. The remaining palette test found that the system-dark fallback lacked
the six new chart colors. The correction gives that fallback the same chart
tokens as explicit dark mode; the existing equality assertion remains intact.
The palette regression is now also in the early focused hosted test gate.
