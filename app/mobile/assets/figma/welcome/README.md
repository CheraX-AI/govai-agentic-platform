# Welcome assets

Source: [GovAI Welcome, frame 278:709](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=278-709).
Downloaded from Figma design context on October 19, 2026. No temporary asset URLs are used at runtime.

| File | Source node | Native slot |
| --- | --- | --- |
| hero.jpg | 278:760 | Full screen, centered cover; includes logo, tagline and coastal artwork |
| next.svg / next.png | I281:721;222:6 | 16 x 16 after Get started |
| globe.svg / globe.png | 296:1979 | 16 x 16 before EN |
| chevron.svg / chevron.png | 296:1984 | 14 x 14 after EN |

Original SVGs are retained unchanged. PNGs are transparent 3x rasterizations
of those SVGs, made with headless Chrome by `scripts/rasterize-welcome-icons.ps1`.
No icons were redrawn and no packages were installed. The script is a development
utility requiring local Chrome, not a runtime or export prerequisite.

The scrim is a native linear gradient: transparent at y=300/772 of screen height,
55% black at its midpoint and 80% black at the bottom. Native text and controls
sit above it. The reference intro has 24px side margins, 12px gaps, a 30px bold
heading, 14px description, 8px spacer and 51px primary button. Its bottom is
10px above the native bottom safe area. Large text and short screens can scroll.
Device frame, simulated status icons, Dynamic Island and home indicator are
excluded; Android owns system chrome. The off-screen onboarding sheet is excluded.
