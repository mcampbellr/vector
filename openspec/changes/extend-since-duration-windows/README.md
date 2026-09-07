# extend-since-duration-windows

Generalize the shared `standup.ParseSince` to accept generic duration windows — `today`, `<N>h`, `<N>d` — instead of the three fixed literals (`24h`/`7d`/`today`), widening `--since` for `vector standup`/`commit` and `since` for `GET /api/activity` with full backward compatibility.
