# Disk cleanup: things you likely don't use (2026-10-11)

Done so far: Docker images/build cache (~38 GB), Go cache (~28 GB), 3 old ollama models (~10 GB). Free: 79 GB.
"Last touched" is the folder's modification date, a hint rather than proof of use.

## A. Almost certainly unused (~110 GB)
| Item | Size | Last touched |
|---|---|---|
| Windows 10 VM `~/Documents/Windows 10 x64 clean.vmwarevm` | 51 GB | Aug 2025 |
| `ollama rm gpt-oss:120b` | 65 GB | pulled 14 months ago, nothing newer |
| ROPA Ubuntu VM | 10 GB | Sep 2023 |
| Ubuntu Server 22.10 VM | 9.5 GB | May 2023 |
| Downloads: 2 Jetson SD images, jetsoninstaller iso (24 GB), tails iso, 3 old IDE dmgs | ~30 GB | |
| devel/tests | 2.9 GB | Feb 2024 |
| Whisper cache, old Hugging Face models | 1.5 + 7.8 GB | Dec 2024 / Jun 2026 |
| Release folders, calcq-prev, devel/tmp, java_error_in_studio.hprof | ~1.2 GB | |

## B. Probably unused, confirm (~85 GB)
| Item | Size |
|---|---|
| X-Plane 12 | 28 GB |
| Simplicity Studio (app 8.5 GB, data 11 GB; app untouched since Jun 2024) | 19.5 GB |
| Android emulator images and AVDs (last changed Sep 20) | 23 GB |
| Cinebench, iMovie, GarageBand, Keynote/Pages/Numbers | ~10 GB |
| Webex, Cisco Spark, zoom support data | ~2 GB |
| Cursor + Kiro + VS Code + WebStorm + CLion + PyCharm (you have many IDEs) | ~15 GB data |
| ollama: qwen2.5-coder:32b, codellama, phi3, gemma, tinyllama | ~27 GB |

## C. Regenerable, safe (~45 GB)
Xcode DerivedData 17 GB, unavailable and old simulators up to 20 GB, CocoaPods 8 GB, Homebrew 1.5 GB, npm 5 GB, gradle caches 9 GB, merged-clean worktrees 7 GB.

## D. Keep
Active repos, zolik-next, kdb, this branch, Claude Code sessions (5 GB), calcq/ai/training until confirmed regenerable.
