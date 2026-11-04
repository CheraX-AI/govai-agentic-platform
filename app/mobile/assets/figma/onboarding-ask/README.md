# Ask onboarding provenance

Figma: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=284-726

Ask hero 284:728 independently specifies a 400 x 382 region, image 100% wide, 169.78% high and top -5.09%. Figma asset 33ac1.png SHA-256 exactly matches the existing onboarding-discover/hero.jpg (985 x 1597). Both arrow SVG hashes exactly match onboarding-meet/back.svg and next.svg, whose existing transparent PNGs are reused at 24 x 24. No temporary URLs are used at runtime.

The responsive implementation uses explicit proportional image dimensions, full-width artwork, measured panel height, and conservative logo bounds. It retains scrolling when native text and the complete logo require more space. Artwork crop adapts vertically instead of shrinking text or adding side gutters. Device mockup chrome excluded.

Official font sources (downloaded October 25, 2026):
- Yaldevi Regular 400: full official variable font from https://raw.githubusercontent.com/google/fonts/main/ofl/yaldevi/Yaldevi%5Bwght%5D.ttf, default weight 400. Retained for the original informational Sinhala chip. Original SIL OFL license preserved as fonts/Yaldevi-OFL.txt.
- Noto Sans Tamil: https://raw.githubusercontent.com/google/fonts/main/ofl/notosanstamil/NotoSansTamil%5Bwdth,wght%5D.ttf . Original variable font, default weight 400 and width 100. License: https://raw.githubusercontent.com/google/fonts/main/ofl/notosanstamil/OFL.txt, retained as fonts/NotoSansTamil-OFL.txt.

No font binary was edited locally. Weight metadata and cmap coverage for both chip labels checked. Android script shaping still needs phone testing. Fonts are bundled and registered with existing expo-font; no dependencies added.
