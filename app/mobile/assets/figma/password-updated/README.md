# Password Updated preview artwork

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2303
Inspected October 27, 2026 with Figma design context and reference screenshot.

`shield-check.png` is the actual shield-check artwork from instance `399:2347`,
component `299:132`. It is not Account Created's circular check or Forgot
Password's lock. The Figma SVG has a 34 × 34 viewBox/root size, green `#006B49`
paths and no background. Its unchanged source was downloaded into the mobile
cache with curl. Installed headless Chrome rasterized it at 4× resolution;
System.Drawing cropped only the 136 × 136 canvas. No paths, colors or SVG root
dimensions were changed. The transparent PNG was visually inspected. It renders
at 34 × 34 inside a centered 72 × 72 pale green `#f2f8f4` badge.

- Source SVG SHA-256: `1b60cd0e76146384fad1f2e48dfec293e2a1c2ff76a68efca75db3e28efaf829`
- Delivered PNG SHA-256: `02bb59e835016e7a6b5f1c1e0c02b5bee43343713f6de0552b702eb1157684f5`

Fresh downloads from this frame of the following assets matched existing source
files byte-for-byte by SHA-256 before reuse:

| Existing file | Figma layer | Geometry |
| --- | --- | --- |
| `../login/top-ribbon.png` | `399:2305` | 272 × 296, cover, x−9/y−14 |
| `../login/skyline.png` | `399:2306` | 723 × 451, contain, x−164, 45% opacity |
| `../login/bottom-ribbon.png` | `399:2307` | 237 × 227, cover, right−7 |
| `../login/logo.png` | `399:2312` | 150 × 151.531 maximum slot; existing nested crop |
| `../register/next.svg` | `I399:2353;222:6` | 16 × 16 white arrow; reuse its existing PNG |

The logo crop remains 154.17% width, 271.32% height, −30.13% left and −63.6% top.
Image slot ratios are preserved when decorative space reduces on narrow/short
windows. Reused arrow PNG provenance/rasterization is documented with Register.
Only this preview's shield is a new runtime asset; no temporary Figma URLs are
used at runtime. Mocked geometry checks inspect effective callsite sizes. Native
visual comparison, crop and TalkBack checks remain pending on a physical device.
