# MediaNinja logo

`medianinja-logo-v2.png` is the current transparent master. The warm white outer
outline keeps the dark head visible on dark backgrounds and at tray sizes.
`medianinja-logo.png` preserves the original unoutlined design. The ninja mask and
right-facing media symbol use the application's acid green / charcoal palette.
Keep the emblem separate from the Paper Mono wordmark so it remains readable
in a tray, taskbar or browser tab.

Generated with the built-in imagegen tool. Prompt brief: "Original MediaNinja
application emblem; geometric ninja mask with an integrated right-facing media
play symbol; neobrutalist, acid yellow-green #ddfa5a and charcoal #141512; bold
forms, transparent background, no text; recognizable at small sizes."

V2 edit prompt: "Preserve the ninja silhouette, bandana, charcoal fill and acid
green media symbol. Add a solid warm white #efeee5 outline around the entire
silhouette, thick enough for small icons. Keep transparency and avoid clipping;
improve contrast against #141512."

After replacing the master, regenerate the checked-in runtime assets:

```sh
pnpm brand:icons
```

This uses the workspace's Tauri icon generator. Web uses the 256px PNG and
multi-resolution ICO; desktop uses the same ICO for the executable/installer,
PNG sizes for windows, and the 32px PNG for the tray. Unused platform variants
stay in `.cache/brand-icons`.

Rebuild the web / desktop application to embed changed assets. Published release
binaries are immutable and receive the new logo in the next release.
