# Register artwork and responsive implementation

Figma: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=398-1962

Inspected/downloaded October 26, 2026. Original Register SVGs are retained unchanged. No temporary Figma URL is used at runtime. SHA-256 hashes and reused backgrounds were verified by an optional local mock regression script; that script is excluded from Git.

| Original export | Local file | Native slot |
| --- | --- | --- |
| 5b714.svg | back.svg / back.png | 20 × 20 inside 38dp visual circle / 48dp touch target |
| 5b467.svg | user.svg / user.png | 18 × 18 |
| 3d6fc.svg | identity.svg / identity.png | 18 × 18 |
| 7f132.svg | mail.svg / mail.png | 18 × 18 |
| d18a1.svg | phone.svg / phone.png | 18 × 18 |
| 6f483.svg | lock.svg / lock.png | 18 × 18, both password fields |
| c8678.svg | eye-off.svg / eye-off.png | 18 × 18 in independent 48dp visibility controls |
| 39224.svg | next.svg / next.png | 16 × 16 |

All eight Register SVGs differed by hash from existing exports, so none were substituted. PNGs are transparent 80 × 80 browser rasterizations using the existing Chrome/System.Drawing approach; viewBox/path data and original SVG files were not edited. Every PNG was visually inspected. The alternate revealed-password state uses the existing original Login eye icon; that state is not specified by the supplied Register frame. Checkbox geometry is native interactive UI; its check is a decorative shape, not scalable text.

Exact SHA-256 matches permit reuse of all four Login artwork files:

- top-ribbon.png: `9a73f4e9db45c52d6ce533623462fff3deda60a0164fce9ffb25b8aee1d0c4dc`
- skyline.png: `869d971b57f896f9da92682d96fd71e6a83618db1bdf2a2b6243a53dc75cff10`
- bottom-ribbon.png: `b7798b7f54574c53b9512a13c76c3223f1b9a10570a2f0284b82e7ae025d1bfe`
- logo.png: `a46897acd200eb92e91654c638af69498a7698ce39d320a71387fcecf6786b48`

The reference uses top ribbon cover 272 × 296 at (-9,-14), skyline contain 723 × 451 at 45% opacity, and bottom ribbon cover 237 × 227. The logo's original nested crop is width 154.17%, height 271.32%, left -30.13%, top -63.6%, inside the reference 118 × 119.204 slot. Runtime keeps that ratio/crop, allows the decorative logo to compact toward 64dp when form space is needed, and preserves a readable vision statement. Wide layouts bound native content to 480dp. Real system chrome/safe areas replace Figma's simulated status bar, device and home indicator.

Approved accessibility/UI-only adaptations include ≥48dp controls/fields, nontruncated labels, darker readable placeholders, disconnected notice, visible unavailable Terms label and optional-consent explanation. The white card retains its 24dp corners, border and shadow. Header and card share one bounded overflow ScrollView (indicator hidden, overscroll/bounce disabled); there is no nested scrolling or forced scrolling when content fits. Header decoration gives way before readable form text. Intrinsic form/vision measurements are independent of flexible card/header sizes and updates are guarded against subpixel repetition. No artwork was repainted.

The form has no backend integration or account creation. All six fields are required. Name/NIC/email/phone use nonblank checks only; passwords use raw nonempty values and exact matching. Strength and identifier policies remain pending. Terms have no approved URL/content and are visibly unavailable/accessibly disabled. The initially unchecked checkbox is an optional demonstration; consent validation is pending. There is no Privacy link or verification destination in this design.

Validation is documented in the mobile README. Native font shaping, visual geometry, TalkBack and keyboard timing still need physical-phone checks.
