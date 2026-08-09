# Episode Parsing Reference

`parseEpisode(title)` in `backend/internal/media/episode.go` returns
(seriesName, season, episode) or ("", 0, 0). Matching order matters:

1. `S<season>E<episode>` (case-insensitive, `\b` delimited), e.g. `S01E01`.
2. `EP<ep>` / `Episode <ep>` / `EP-1` styles, e.g. `EP3`.
3. Bare `E<ep>`, e.g. `Show E03`.
4. CJK `第<ep>集|話|话`, e.g. `城市猎人 第1集`.
5. Mid-title number `Name<NN>...` (2-3 digits) only if the prefix contains a
   letter/CJK character, e.g. `封神榜01女娲宫风波` -> (`封神榜`, 1).
6. Trailing `(NN)` or `[NN]`, e.g. `1 (4)`.
7. Trailing `  NN` / `-NN` / `_NN` (1-3 digits), e.g. `The Room 101`.

The number guard in rule 5 excludes date-like titles such as
`2024 2 12 利哥探花 黑丝` (prefix has no letter) and `星际穿越 Interstellar 2014`
(year is 4 digits, not 2-3).

## Grouping rules (rebuildSeries)

- Key = `lower(seriesName) + "\x00" + season`.
- A group becomes a series only when it has >=2 episodes.
- Series with fewer than 2 available episodes after a rescan are deleted.
- Episodes sort by season, then episode number.

## Tests

`backend/internal/media/episode_test.go` covers:

| title | series | season | ep |
| --- | --- | --- | --- |
| 1 (4) | 1 | 0 | 4 |
| 星际迷航 S01E01 | 星际迷航 | 1 | 1 |
| 城市猎人 第1集 | 城市猎人 | 0 | 1 |
| 封神榜01女娲宫风波 | 封神榜 | 0 | 1 |
| Show E03 | Show | 0 | 3 |
| SSIS-698 | SSIS | 0 | 698 |
| 星际穿越 Interstellar 2014 | (none) | 0 | 0 |
| 2024 2 12 利哥探花 黑丝 | (none) | 0 | 0 |
| The Room 101 | The Room | 0 | 101 |

When changing parsing behavior, extend these cases first and keep them green.
