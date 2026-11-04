# Meet GovAI assets

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=686-4783

- hero.png: original Figma illustration, Hero node 686:4785. Display with contain in a 400:372 slot; no stretching or redrawing.
- back.svg: original Back icon, node 686:4806.
- next.svg: original Next icon, node 686:4813.
- back.png and next.png: transparent 72 x 72 rasterizations of those original SVGs for 24 x 24 native slots. Generated with headless Chrome and System.Drawing using the existing welcome-icon conversion approach. SVG sources are retained unchanged; no packages installed.

All runtime assets are bundled local files. Temporary Figma download URLs are not used at runtime. Device chrome is excluded. Native text below the illustration remains scalable; text embedded in the original illustration is unchanged.
