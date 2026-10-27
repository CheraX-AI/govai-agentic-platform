# Check Email preview artwork

Figma frame 399:2144, inspected October 26, 2026:
https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2144

The mail/check SVG export 035ff.svg is preserved unchanged as mail.svg. SHA-256:
`cdbae6793b123dbb6e5e5f7bc111d84df0fd7eaa03563a07d5d030ae96620bcb`.
mail.png is a transparent 136 x 136 browser rasterization, visually inspected,
displayed at 34 x 34 inside the frame's 72 x 72 pale-blue badge. The source color
is #2D6FA6. No new package or temporary runtime asset URL is used.

All four frame backgrounds/logo match existing Login assets by SHA-256. Its
Back SVG matches Register's back.svg. Those assets are reused without changes.
The logo uses the frame's own 150 x 151.531 slot and the existing verified crop
ratios (154.17% width, 271.32% height, left -30.13%, top -63.6%). Native system
bars replace simulated device chrome.

The compact white card keeps 20-dp padding, 12-dp section gaps, 4-dp text gaps,
24-dp corners and the original blue badge. Its height is intrinsic, never fixed.
The concise "Preview · No email sent" notice flows between the logo and card;
The footer keeps Use a different NIC without a visible Exit preview control.
The static "Didn’t get it? Resend" row has an adjacent unavailable caption and is
accessibly disabled text without a redundant button-sized row. Back has a visible
38-dp white circle and an in-bounds 48-dp target above the artwork. All active
controls retain at least 48-dp targets; scalable text and hidden-indicator overflow
scrolling remain. Measured card/notice/vision updates ignore identical dimensions.
The disabled mailbox button keeps a burgundy fill, white text, an adjacent
unavailable caption, and its accessibility disabled state; it has no handler.
Logo width targets 37.5% of the logical window, capped at 150 dp and constrained
by the vision text. Decorative header space reduces first on short windows.
Artwork proportions/crop ratios are unchanged; anchor offsets scale with images.
Approximate default card height is 383 dp before wrapping (not a rendered result),
versus the reference's 362 dp; honest preview copy and accessible controls add space.

This is only a development sample. The email is the fixed sample@example.test,
not a lookup or entered identifier. No email was sent and no reset link exists.
Mailbox and Resend have no handlers and are accessibly disabled. The design's
sent-email claim, expiry claim and resend countdown are replaced by approved
disconnected copy. No timers, requests, tokens or mailbox actions are added.
Back, Android Back and Use a different NIC open fresh Forgot Password.
Forgot Password submission is unchanged and never opens this preview.

Both the App entry and direct screen mount require __DEV__ and the dedicated
flag. No static import or startup preload pulls this exclusive asset into
production. The existing Account Created preview keeps its own separate guard.
Automated layout checks are synthetic, not physical-device rendering.
