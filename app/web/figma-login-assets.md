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

The emblem is a 455×383 PNG. The photo is a 985×1597 JPEG, despite Figma serving it under a .png URL; only its local filename extension was corrected. The photo panel crops the original with image width 122.26%, height 162.6%, left -19.56%, top -62.6%, and a transparent-to-black scrim from 45% to 100%. The original is retained for provenance; the current proportional coastal derivative is described below.

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

## GA-52 NIC/email increment - 26 October 2026

Reference: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=635-2405. Added unmodified exports for inactive phone method, active NIC/email method and mail input; all 18x18. Reuses the existing page/photo/fonts and controls; no temporary runtime URLs.

- mobile-tab-inactive.svg: export 33c6b.svg; SHA-256 6876f494dcabad6c0d4eafd2b1bcaa29b5cd2500c6d548274e187f8dbb439570.
- nic-email-tab-active.svg: export 23fd9.svg; SHA-256 d52ace1c73793cd9944bd6ac0218a79850a1a9aebec7f2fb3c6476c54b1c313e.
- mail.svg: export 28387.svg; SHA-256 8de1be7da9ad8df09179983d94c0945698e8a8ce49c4fecbb6a27ce99eaca875.

## GA-52 registration assets - 26 October 2026

Frame 639:2509 supplies name/NIC/password-hint icons; existing photo, brand, phone/mail/lock/eye and submit controls are reused. New exports are unmodified.
- register-user.svg: export 7f4bf.svg; slot 18x18 name; SHA-256 3d68f57a1d9d60dea7237ce3005fc4513eb47f94e23037b29bd9151c82aa2b88.
- register-nic.svg: export 3d6fc.svg; slot 18x18 NIC; SHA-256 c183b8009f2fb25ffb5037e8535bd8e294c8b16a701b2459688f310ea5276d9a.
- register-info.svg: export 9ed54.svg; slot 14x14 hint; SHA-256 83dc0d6ff96b30a54ca1c2bb0e3d34ad29b3fe8f4e05d77803abfb35abc22e91.

## Photo crop correction - 27 October 2026

The original login-photo.jpg remains 985x1597 and unchanged. The earlier height162.6%/bottom-centre workaround could hide the flag and lighthouse top on wide panels. It is superseded by auth-coast.jpg: a deterministic rectangle (x0,y620,width985,height977) cut from that original without resizing or generated content. This removes the upper logo outside the reference's coastal composition. The derived JPEG SHA256 is DA1F384EC3CD33945340B09642FD88B9141601D2B2BE124D0C4D29ABDDB1E1E2.

Current AuthLayout uses the derivative with width100%,height100%,inset0,object-fit:cover,object-position:100% 0%. The panel remains viewport-height/sticky, independent of form height. Top/right prioritises the full flag and lighthouse top; tall/narrow panels crop left sea and wide panels crop lower rocks. Source proportions remain uniform. Caption/help overlays remain unchanged. This is a proportional adaptation of Forgot password frame635:2583, not a claim to reproduce Figma's nonuniform transformed source exactly.

## Recovery confirmation icon ? 27 October 2026
- Source: file IuwQ09eZ1P5ctrc1vkVgEz, frame 635:2669, mail-check node635:2745. Official MCP SVG export f78ef.svg downloaded locally as recovery-mail-check.svg, natural34x34. SHA256: 7C16176F599149310BBCC2F5F7A48CAF749DBF357F5C53C1DA9C2DECAE0964FD. Temporary download URL is not used at runtime.
- Existing register-info.svg and register-nic.svg reused for shared information/NIC controls. Original emblem/photo and font fallbacks unchanged.
