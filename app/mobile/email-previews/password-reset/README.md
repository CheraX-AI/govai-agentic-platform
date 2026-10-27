# Password reset email — disconnected preview

This is a standalone browser preview of an HTML email, not a React Native
screen or a connected password-recovery implementation. No email is sent, no
token is generated, no password changes, and the disabled Reset password button
has no URL, event handler or action. There are no scripts, forms, remote fonts,
remote images or tracking pixels in the HTML. Only a local PNG is loaded.

## Open on Windows

From `app/mobile` in PowerShell:

```powershell
Start-Process (Resolve-Path 'email-previews/password-reset/index.html').Path
```

Keep `assets/govai-logo.png` beside the HTML in its current relative location.
No server, package install, Expo flag or app route is needed. The plain-text
counterpart is `password-reset.txt`.

## Design and adaptations

Reference: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2380

The reference shows an email inside a mail client. Phone chrome, Inbox/Back,
sender avatar, mail timestamp and mail-client divider are excluded. The pale
rounded card, centered logo and burgundy CTA treatment are retained. The HTML
uses presentation tables, inline essential card/button styles, system fonts,
fluid width, a 400px maximum content width and smaller narrow-window padding.
Text grows naturally; the page scrolls rather than shrinking the entire layout.

The preview adds a prominent disconnected notice, generic greeting and honest
sample body. It removes the reference's personal name, NIC suffix and unverified
sender address. The expiry, sender, production guidance and proposed footer are
explicitly pending backend/team confirmation. The proposed footer and logo are
design material, not evidence of government endorsement or sender verification.

Logo provenance: direct PNG export of email-frame logo node `399:2389`, retrieved
through Figma on October 27, 2026. Native export dimensions: 96 × 97 pixels.
Visually inspected against the email reference; no redraw, crop, recoloring or
resampling was applied. HTML preserves its aspect ratio. Verification here means
matching the design asset only; brand authorization/official status is unverified.
SHA-256: `826B92CB43C10FEE4D7D3D1A6450299F7F3D67BFA30B5AFDEF66EAA7F43E6E95`.
The expiring download URL is not used at runtime. A higher-resolution approved
asset may be needed for production high-density rendering.

## Placeholder URL — documentation only

`https://example.invalid/reset-password/PLACEHOLDER`

This reserved invalid-domain example is not embedded in either message file.
It is not a real reset URL and must never be sent as a working recovery link.
The preview button deliberately remains a disabled button, not an anchor.

## Shared backend developer handoff

Hand over these four files plus the Figma reference after copy and team review.
The HTML is a visual starting point, not a production-ready sending template.
The backend developer will adapt it to the shared backend's template engine,
keep the plain-text version aligned, replace the disabled preview control with
an email-compatible anchor, and remove preview-only material only when the flow
is connected and the production copy is approved. No backend implementation is
included or authorized by this preview task.

Agree the template contract before integration. Suggested concepts (not an
existing API): validated reset URL, confirmed expiry display, approved sender,
reply-to and support details, and an optional safely escaped greeting. Do not
require a name or NIC suffix by default. Backend owns account lookup, token
creation/storage/validation, expiry, single use, delivery and abuse controls.
URL values require a trusted HTTPS destination and correct HTML attribute
escaping. Avoid putting recipient details or tokens into logs or sample files.
Decide separately whether the destination is a web password form or a mobile
universal/app link, including fallback when the app is absent. Specify invalid,
expired, already-used and successful reset behavior.

Outstanding confirmations: actual destination/route, template variable names,
30-minute expiry policy, subject, sender identity/domain, reply-to, support,
footer/branding authorization, production guidance, logo hosting versus CID,
localization, supported clients, dark mode and accessibility acceptance criteria.
Production delivery needs both HTML and plain text and email-client QA.

## Validation performed — October 27, 2026

- Actual headless Chrome rendered the HTML in same-origin iframe viewports of
  320, 400 and 800px, plus 320px with text enlarged to 200% through a test-only
  stylesheet. These were browser rendering checks, not calculated layouts.
- DOM checks before adding test diagnostics found no horizontal overflow in all
  four cases, confirmed the logo loaded at its original ratio, zero anchors and
  a disabled CTA. The wide card stayed bounded; default text remained 14px and
  enlarged body text was 28px. The enlarged case naturally required scrolling.
- Visually inspected a composite browser screenshot for wrapping, spacing,
  card/button presentation and logo proportions against the 400px reference.
  Added safety notices intentionally make the content taller than Figma.
- Source inspection found no live URLs, personal reference details, scripts,
  forms or action handlers in either message. The placeholder is documentation
  only. Burgundy `#8b1a2e` with white button text exceeds WCAG AA contrast;
  muted body copy uses `#595959` on the pale card. The disabled button is omitted
  from keyboard tab order by native HTML behavior; there are no active controls.
- Temporary browser harnesses/profiles/screenshots were stored in the system
  temporary directory, outside the delivered files. No dependencies were added.
  No mobile source, navigation, package files or preview guards were edited.

Not performed: Gmail/Outlook/Apple Mail or other actual email-client testing,
physical Android testing, screen-reader testing, real browser zoom/system-font
testing, dark-mode testing, delivery, token creation or backend integration.
The 200% stylesheet check simulates enlarged text; it is not system-font testing.
Browser rendering does not establish email-client compatibility. Repeat client
QA after backend adaptation, including image blocking, narrow screens, enlarged
text, keyboard/screen-reader use, dark mode and HTML/plain-text consistency.
