# Figma assets

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=47-3

Downloaded unchanged using the asset URLs returned by Figma `get_design_context`.
No expiring URLs are used at runtime.

| File | Figma node | Reference display slot |
| --- | --- | --- |
| top-ribbon.png | 47:7 | x -15, y -12, 272 x 296; cover |
| skyline.png | 47:9 | x -164, y 379, 723 x 397; opacity .45; nested image crop |
| bottom-ribbon.png | 47:8 | x 170, y 605, 237 x 167; cover |
| logo.png | 52:9 | x 56, y 199, 312 x 315; nested image crop |

The original logo image is a full reference composition used as a cropped image
fill in Figma. App.tsx reproduces Figma's final fill crop (154.17% width,
271.32% height, -30.13% left, -63.6% top) in a clipped native View; it does not
display the full composition. Figma places this opaque logo layer above the
progress track, obscuring it. This layer order is preserved.

The screen removes simulated phone chrome, subtracts its 30 px top and bottom
areas, and fits the resulting 400 x 712 artwork into Android's real safe area.
Logo and loading content remain together; the skyline and bottom ribbon stay
anchored to the bottom. Original aspect ratios/crops are preserved at each scale.
The native Android system provides the status and navigation indicators.
