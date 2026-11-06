# Disk cleanup plan (2026-10-11)

Free now: **3.1 GB**. Nothing below has been deleted; this is a survey plus a proposed order.
Sizes are `du` figures; the stage totals are estimates.

## Where the space is

| Area | Size | Notes |
|---|---|---|
| ~/Library/Caches/go-build | 29 GB | Go build cache; regenerates |
| Docker (images, volumes, build cache) | 43 GB on disk | 15.6 GB images, 4.6 GB volumes, 21.5 GB build cache all reclaimable |
| ~/Downloads | 41 GB | Jetson images 24 GB, tails iso 1.8 GB, installer dmgs ~3 GB, audiobook 2 GB |
| ~/Library/Developer/CoreSimulator | 24 GB | Stale simulators and runtimes |
| ~/Library/Developer/Xcode/DerivedData | 17 GB | Old builds; regenerates |
| ~/Library/Android/sdk | 19 GB | Keep unless old system images are in it |
| ~/Library/Application Support/Claude | 15 GB | Per the app: Code sessions 5.2 GB, web caches 1 GB |
| ~/.cache (huggingface 7.8 GB, uv 1.8, whisper 1.5) | 11 GB | Models re-download |
| ~/Library/Caches/CocoaPods | 8.3 GB | Regenerates |
| ~/.gradle + ~/.android | 20 GB | Caches and AVDs |
| devel/calcq | 28 GB | ai/training 19 GB, node_modules 6 GB (not zolik) |
| devel/zolik worktrees (19 listed) | ~14 GB | See below |

## Worktrees (git, confirmed against origin/main)

Merged and clean (safe to remove): zolik-lanweb **5.7 GB**, zolik-overnight 488 MB, zolik-qa 517 MB, zolik-sound 491 MB, zolik-rules-button 28 MB, zolik-standin 28 MB, .claude/worktrees/mobile-hints-log 28 MB.

Merged but with uncommitted changes (inspect `git status` first): zolik-overnight-ios 1.4 GB, zolik-macos 725 MB, zolik-overnight-mac 490 MB, zolik-raise 482 MB, zolik-scroll-to-round-end, .claude/worktrees/{game-zoom-scroll-table, optimistic-bhabha, practical-sutherland}.

NOT merged, keep: zolik-next (seat-badges, PR #310), zolik-admin-ui, .claude/worktrees/legal-notices, and this checkout (canastax-drag-melds, 30 dirty files).

Also: devel/grpc-bench/zolik and devel/release-1.6.2/zolik are detached-HEAD worktrees; devel/calcq-prev (420 MB), devel/tmp (349 MB), and the release-1.* folders are probably dead.

The Claude app reports 3 "kept ready" worktrees (2.8 GB) that it reaps itself, and `clean_up_worktrees` can remove app-made worktrees idle 30/60/90 days.

## Proposed order (safest first)

1. **Docker prune**: `docker builder prune -af`, `docker image prune -af`, `docker volume prune -f` (check the 5 volumes in use first). About 35-40 GB. Restarting the dev stack rebuilds images.
2. **Go build cache**: `go clean -cache`. About 29 GB. The next build is slow once.
3. **Xcode**: `xcrun simctl delete unavailable`, delete DerivedData, and remove simulators for iOS runtimes you no longer test (18.6 / 26.0 are candidates, 26.5 is current). 17 GB plus up to ~20 GB.
4. **Merged-clean worktrees** with `git worktree remove` plus a branch delete: about 7.3 GB, mostly zolik-lanweb.
5. **Downloads**: the Jetson SD images and the installer dmgs. About 30 GB. Your call, since I can't tell what you still need.
6. **Dev caches**: CocoaPods, Homebrew (`brew cleanup`), ~/.npm, ~/.gradle/caches, huggingface models you don't use. About 15 GB.
7. **Dirty merged worktrees**: after reviewing their uncommitted changes. About 3.5 GB.
8. **calcq/ai/training (19 GB)**: only if you confirm it's regenerable.

Stages 1-3 alone should free roughly 80 GB with no risk to source. Remove nothing in stages 4-8 without your say-so.

## Update: whole-disk survey (Data volume is 861 GB used of 926 GB; 41 GB free after Docker prune)

Docker prune (build cache + unused images) freed ~38 GB of real disk. Docker.raw is now 5.4 GB.

Largest items outside the repo work (the earlier list missed these):

| Item | Size | Note |
|---|---|---|
| ~/.ollama | 123 GB | `gpt-oss:120b` alone is 65 GB (last used 14 months ago); qwen2.5-coder:32b 19 GB, qwen3.6:27b 17 GB |
| ~/Documents/Windows 10 x64 clean.vmwarevm | 51 GB | VMware VM |
| ~/Virtual Machines.localized | 20 GB | ROPA Ubuntu 10 GB, Ubuntu Server 22.10 9.5 GB |
| ~/X-Plane 12 | 28 GB | |
| ~/SimplicityStudio + /Applications/Simplicity Studio.app | 11 + 8.5 GB | |
| ~/Library/Application Support/Cursor | 7.7 GB | plus ~/.cursor 4 GB |
| ~/Library/Arduino15 | 7.4 GB | |
| /private/var/folders | 16 GB | temp dirs; clears on reboot |
| ~/models (gguf) | 4.2 GB | duplicates of ollama models? |
| ~/.konan, ~/.minikube, java_error_in_studio.hprof | 3.6 GB, 1 GB, 335 MB | hprof is a crash dump, safe to delete |

Suggested additions: remove unused ollama models (`ollama rm`), start with gpt-oss:120b and other 4+ month-old ones (~100 GB); decide on the Windows 10 VM and two Ubuntu VMs (~70 GB).
