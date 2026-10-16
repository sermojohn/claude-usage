# Journal: 2026-10-16-poll-current-session-missing

<!-- fr:journal kind=repro scope=debug id=48654ed08501 created=2026-10-16T10:07:12+00:00 -->
### 48654ed08501 · repro · Not reproducible now: Session row present in every poll frame

Report: in polling mode the current session is not shown. Tried at 2026-10-16 ~13:05 EEST with both main@8981a7a and the installed ~/.local/bin/claude-usage (built Oct 14 15:00), default flags, `--polling-interval-seconds 3`, under a pipe and under a real pty (`script -qF`, TERM=xterm-256color). Every frame was `ESC[H ESC[2J` followed by the Session and Weekly rows. Raw `claude --print "/cost"` matched sessionRe (`Current session: 11% used · resets Oct 16 at 3:10pm (Europe/Athens)`).

<!-- fr:journal kind=ruled-out scope=debug id=f50988c98248 created=2026-10-16T10:07:15+00:00 -->
### f50988c98248 · ruled-out · Poll loop / render path differs from single-shot

poll() calls the same doFetch closure as single-shot mode (fetch -> Parse -> Render), with the same showSession/showWeekly. Byte captures of both modes show identical rows; the only difference is the clear-screen prefix.

<!-- fr:journal kind=ruled-out scope=debug id=8078472cfaa6 created=2026-10-16T10:07:17+00:00 -->
### 8078472cfaa6 · ruled-out · Terminal clear sequence or lipgloss TTY behaviour eats the first row

A pty capture shows the Session row written right after `ESC[H ESC[2J` on every tick; the claude subprocess writes nothing to the tty.

<!-- fr:journal kind=hypothesis scope=debug id=1d4406f33621 created=2026-10-16T10:07:20+00:00 -->
### 1d4406f33621 · hypothesis · UNCONFIRMED: session line changes format once the 5h window resets; the parser silently drops it

sessionRe only accepts `N% used · resets <Mon D> at <time> (<TZ>)`. If the session line takes another shape (e.g. no active window after a reset, with no `resets` clause), Parse leaves Session nil with no error, because Weekly still matched, so found=true. The installed binary (before 8981a7a) then omits the row entirely, and main shows `--`. A long-running poller is exactly what crosses a window reset, whereas one-off runs usually happen mid-session. I have not captured `claude --print "/cost"` output in the no-active-session state to confirm this.
