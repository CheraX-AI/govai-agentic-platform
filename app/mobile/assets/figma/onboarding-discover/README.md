# Discover assets

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=278-761

Figma hero node 278:812 uses a 400 x 382 slot, width 100%, image height 169.78%, top -5.09%, left 0, with clipping. Original hero.jpg is 985 x 1597. Its proportional height at width 400 is 648.53, matching the Figma crop rectangle 648.56 within rounding. Explicit dimensions and contain preserve proportions. The panel overlaps by 42 logical units.

Original service SVGs: identity (279:720), vehicles (279:726), certificates (279:732), passports (279:738). Transparent PNGs are 72 x 72 rasterizations for 24 x 24 slots, created with headless Chrome and System.Drawing using the existing welcome-icon approach. SVGs retained unchanged. No new dependencies.

Back and Next reuse exact matching original assets in onboarding-meet (Figma asset names a7580.svg and d06bd.svg). No temporary URLs are used at runtime. Native text scales; embedded artwork text is unchanged. Device mockup chrome is excluded.
