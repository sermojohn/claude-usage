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
