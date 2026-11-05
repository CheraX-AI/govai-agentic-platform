# Check Email preview artwork

Figma frame 399:2144, inspected October 26, 2026:
https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2144

The mail/check SVG export 035ff.svg is preserved unchanged as mail.svg. SHA-256:
`cdbae6793b123dbb6e5e5f7bc111d84df0fd7eaa03563a07d5d030ae96620bcb`.
mail.png is a transparent 136 x 136 browser rasterization, visually inspected,
originally displayed at 34 x 34 inside the frame's 72 x 72 pale-blue badge.
The source color is #2D6FA6. The approved supplied-reference adaptation displays
the same PNG at 32 x 32 with tintColor #007a53 inside a 64 x 64 #edf6f1 badge.
The PNG/SVG bytes are unchanged. No new package or runtime asset URL is used.

All four frame backgrounds/logo match existing Login assets by SHA-256. Its
Back SVG matches Register's back.svg. Those assets are reused without changes.
The logo uses the frame's own 150 x 151.531 slot and the existing verified crop
ratios (154.17% width, 271.32% height, left -30.13%, top -63.6%). Native system
bars replace simulated device chrome.

The supplied image guides the current layout rather than its phone border,
camera island, status icons or black home indicator. No app-rendered home-indicator
decoration was found or added. The white card uses 16dp padding, 8dp section gaps,
4dp header text gaps and 24dp corners. Its height is intrinsic, never fixed.
The no-email-sent notice now sits inside the card under the heading. The help
panel reuses assets/figma/forgot-password/info.png, and the active burgundy login
button reuses Register's next.png white arrow. No image files were modified.
Resend is a disabled link with an unavailable caption and no handler. All controls,
including the disabled Resend target, retain at least 48dp targets. The original
Back arrow has a 38dp circle within its 48dp target. Scalable text, safe areas
and hidden-indicator overflow scrolling remain.

The width-safe logo targets 37.5% of the logical window, capped at 150dp and
constrained by the vision text. Its original crop and image aspect ratio are
unchanged; short windows scroll rather than shrinking the logo. Original opaque
white pixels in the logo composition are retained. Artwork offsets scale with
their images. At 400 × 772 with simulated 24dp top/bottom safe-area insets,
the corrected HTML simulation measures a 435px card and no overflow. Both modes
passed six viewport sizes at text scales 1/2/3 plus two short-window cases.
These browser measurements do not establish native Android rendering or actual
phone fit. Physical Android/system-font/TalkBack testing remains pending.

This is only a development sample. The email is the fixed sample@example.test,
not a lookup or entered identifier. No email was sent and no reset link exists.
Resend has no handler and is accessibly disabled. The explicit notice is
“Demo preview — no email was sent.” No expiry claim, countdown, request, token
or mailbox action is added. Back to log in opens fresh Login while retaining
its original return destination. Header Back, Android Back and Use a different
NIC open fresh Forgot Password. Only the existing connected-development Email
submission with nonblank NIC and the required simulated checkbox opens this
demo; normal mode and Phone remain disconnected on Forgot Password.

Both the App entry and direct screen mount require __DEV__ and the appropriate
standalone or connected-demo flag. No static import or startup preload pulls this exclusive asset into
production. The existing Account Created preview keeps its own separate guard.
Automated layout checks are synthetic, not physical-device rendering.
