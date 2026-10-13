# Changelog

## Unreleased

- Engine settings persist across restarts. Successful changes are written to
  settings.json next to the task history file; a missing or corrupt file
  falls back to CLI defaults.

## 0.1.2

- Batch downloads are a generic source registry: the engine announces batch
  sources via `/api/session`, the UI renders a source-agnostic batch page, and
  the API accepts `batch: {site, names}` requests.
- Downloads support retries. A finished task can be retried in full, or with
  RETRY FAILED only the failed files are fetched again while already collected
  files are skipped.
- Task records persist across restarts. Finished tasks are stored in the
  per-user config directory (`serve --state` overrides the path) and can be
  deleted individually or cleared in bulk from the history panel.
- The queue groups running downloads first and collapses finished tasks behind
  a FINISHED toggle; collected file rows inside a task collapse the same way.
- Task cards show aggregate file progress (12 / 47 FILES · 2 FAILED) driven by
  a new plan event, and history records are collapsible with a summary row.
- The workspace fits the viewport in one screen: columns scroll internally
  instead of paging, and task-level progress is a segmented bar that reads
  differently from the compact per-file rows.
- File progress events are keyed by base filename, so a file can no longer
  appear twice on a task card (once as a download row, once as collected).

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
