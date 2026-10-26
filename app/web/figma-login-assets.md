# GovAI login design asset inspection

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=613-2372
Frame: `613:2372`, “01c - login / mobile”.
Inspected: 2026-10-15. Actual frame is 1440×820 with a 600px form panel and 840px photo panel.

## Retrieved original assets

All 13 assets were downloaded from the asset URLs supplied by Figma design context. Files are non-empty; SVG roots and raster formats were checked. No artwork was recreated or substituted. Runtime base path: `/assets/figma/login/`.

| File | Figma node / slot | Usage |
| --- | --- | --- |
| carousel-arrow-right.svg | 371:196 / 613:2471 | Next slide; 24×24 SVG inside 22×22 design wrapper |
| carousel-arrow-left.svg | 242:72 / 613:2467 | Previous slide; 24×24 SVG inside 22×22 design wrapper |
| govai-emblem.png | I627:2287;514:195 / 808:8648 | Header 57×48; help badge 40×40; contain |
| login-photo.jpg | 613:2458 | 840×820 photo panel; source 985×1597 |
| globe.svg | 613:2381 | Language selector; 18×18 |
| chevron-down.svg | 613:2386 | Language selector; 16×16 |
| mobile-tab.svg | I613:2395;263:95 | Active mobile tab; 18×18 |
| nic-email-tab.svg | I613:2402;263:100 | NIC/email tab; 18×18 |
| phone.svg | I613:2408;242:161 | Mobile input; 18×18 |
| lock.svg | I613:2419;242:161 | Password input; 18×18 |
| eye.svg | I613:2419;242:165 | Password visibility; 18×18 |
| shield-check.svg | 613:2442 | Security check; 22×22 |
| login-arrow-right.svg | I613:2446;530:211 | Login button; 18×18 |

The emblem is a 455×383 PNG. The photo is a 985×1597 JPEG, despite Figma serving it under a .png URL; only its local filename extension was corrected. The photo panel crops the original with image width 122.26%, height 162.6%, left -19.56%, top -62.6%, and a transparent-to-black scrim from 45% to 100%. Keep the original image and reproduce crop/overlay in CSS.

## Missing or not supplied

- Font binaries: Figma specifies Inter (400/500/600/700), Montserrat ExtraBold (800), and Fraunces Bold (700, SOFT 0, WONK 1). No downloadable font files were supplied; matching files were not found in the inspected web project. The preview uses system sans-serif and Georgia fallbacks; font sourcing remains unresolved.
- Additional carousel slides: three indicators and navigation are visible, but this frame supplies only the current photo and caption. Slide 2/3 photos and copy were not supplied by this frame; availability elsewhere in the Figma file is unverified.
- No retrieved image or SVG is currently inaccessible.

## Implementation notes

The wordmark is styled text, not a missing logo image: Montserrat ExtraBold 26px, letter spacing -0.5px; Gov gradient #a30f2d to #80040f, AI gradient #f5a915 to #d97706. Subtitle uses Inter 13px.

Welcome heading uses Fraunces Bold 26px. Main text color #142033, secondary #444, tertiary #666, primary accent #a30f2d, border #e5e7eb. Login card is 440×563 at x80/y143.5, radius 20px, 32px horizontal padding, 28px vertical padding.

Checkboxes, slide indicators, panel scrim, borders and shadows are native design shapes/styles; they do not require separate image assets. The security check supplies visual UI only, not a working CAPTCHA integration.

Preserve distinct icon variants (especially carousel and login arrows). Preserve exported SVG root dimensions; the carousel SVGs are 24px while their design wrappers are 22px.

This note originally recorded asset retrieval only. The phone-login preview now uses these assets; its browser check verifies that all rendered images load. Carousel arrows and the language chevron are retained as original design assets for deferred controls. See README.md for current implementation limits.
