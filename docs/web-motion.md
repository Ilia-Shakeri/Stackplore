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

All execution and interaction checks run on GitHub Actions. Fresh real-browser
checks of rapid reopen, toast replacement/swipe, both locales, keyboard focus,
reduced motion and reduced transparency remain required; source review is not
visual or device-performance acceptance. These changes do not finish stable 2.0.
