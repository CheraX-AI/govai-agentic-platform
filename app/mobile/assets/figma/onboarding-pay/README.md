# Pay onboarding assets and geometry

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=284-908

Pay independently specifies display 400 x 772, hero 400 x 382, panel y=340 / height 432, overlap 42. Its image fill is width 100%, height 169.78%, top -5.09%. Native layout maps the reference's simulated 30-unit top/bottom system bands into actual safe areas; image proportions and complete logo are protected, native content wraps, overflow scrolls, and surplus height stays before the footer in the white panel.

Hero 33ac1.png and arrow sources a7580.svg / d06bd.svg were downloaded to ignored cache and SHA-256 matched against existing onboarding-discover/hero.jpg and onboarding-meet SVGs. Existing matching JPEG hero and PNG arrows are reused. No temporary Figma URLs are used at runtime.

New original SVG assets retained unchanged:
- payment.svg: 900b5.svg, green credit-card icon, 24 x 24 native slot.
- application.svg: c4cb7.svg, burgundy file icon, 24 x 24.
- clock.svg: ddfe0.svg, grey clock, 12 x 12.
- meta.svg: 2a216.svg, unread marker and chevron, 30 x 30.
Transparent 90 x 90 PNGs were rasterized with headless Chrome and System.Drawing using the existing project approach. Native square slots preserve their proportions. All four PNGs visually inspected. No packages installed.

Notifications are informational native text/Views, with no actions, permissions, financial state or backend requests. The sample amount and GovAI Plus wording are design copy, not an actual charge or product promise. Five decorative onboarding indicators replace Figma's four by explicit approval.
