# Changelog

## 0.1.1

- Settings save automatically when a field loses focus or Enter is pressed.
  Folder selection applies immediately. Switching tabs, closing settings or
  starting a download also commits pending changes; the Save Settings button
  has been removed.
- Invalid values stay editable without changing engine settings. Saving errors
  are visible and retryable, and serialized writes protect newer edits from
  stale responses. Existing downloads retain their original settings.
- New MediaNinja logo across the Web UI, favicon, Windows executable and tray.
  A warm white outline keeps the ninja silhouette visible on dark backgrounds.
