# Chat Home menu assets

Source: [GovAI chat home2 – Menu open, frame 306:1388](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=306-1388), drawer instance 306:1501.
Inspected/exported October 28, 2026 with the Figma connector.

Original emblem PNG and close, new-chat, recent, chevron, applications, history,
more and settings SVGs came from this drawer's design-context asset exports.
The nested brand instance could not be exported directly; wordmark.png is a 3x
PNG export of original wordmark node 514:197. It is paired with the original
emblem at the inspected 57x48 / 80x26 sizes and 12dp gap, total149x48.
This preserves the original gradient wordmark without adding a font dependency.

SVGs remain unchanged as source files. PNG icons are 3x transparent rasterizations
using `scripts/rasterize-chat-home-menu-icons.ps1` (existing Chrome and Windows
System.Drawing). Native Image uses explicit 26dp primary, 20dp history/close,
16dp chevron and 22dp settings dimensions with contain sizing. sha256.json records
original and raster file hashes. No generated replacement artwork is used.

The approved neutral footer reuses ../chat-home/profile.png at 22x22 inside a 40dp
disc, replacing Figma's personal initials/name/plan with Citizen / Development
preview. No name or account details are inferred. Existing Inter fonts are reused.
