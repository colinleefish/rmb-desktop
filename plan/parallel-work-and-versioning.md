# rmb-desktop 并行工作规范：分支（Branching）+ 版本（Versioning）

> 制定日期：2026-08-15。适用对象：所有并行运行的 AI agent（pi / cursor / claude / codex / workbuddy 等）以及人工提交。
> 背景：多个 agent 同时在主 checkout 和 git worktree 上工作，出现了未合并分支、脏工作树、版本号不一致、发布文档过时等问题。本文档是后续并行协作的硬性约定。

---

## 1. 总原则

1. **main 是唯一发布基线**。所有功能最终合入 main，只有 main 可以打 tag、发 release。
2. **一个任务 = 一个分支 = 一个 worktree = 一个 agent 会话**。任何 agent 不得在主 checkout（`/Users/liguanghui/Virginia/colinleefish/rmb-desktop`）直接改代码；主 checkout 只用于 review、合并、发布。
3. **分支从最新 origin/main 切出**，除非任务明确说明依赖另一个未合并分支（此时必须把依赖分支名写进任务描述，并在合并时先合依赖）。
4. **不做历史改写**：合并一律用 merge commit（不 squash、不 rebase 已推送分支），保证每个 agent 的工作在历史上可追溯。
5. **谁提交谁负责**：合并动作只能由用户本人，或用户明确指定的"集成会话"执行。

---

## 2. 分支规范

### 2.1 命名

| 类型 | 前缀 | 例子 |
|---|---|---|
| 功能 | `feat/<slug>` | `feat/workbuddy-integration` |
| 修复 | `fix/<slug>` | `fix/kill-listeners-port` |
| 重构（行为不变） | `refactor/<slug>` | `refactor/simplification` |
| 性能/调参 | `tuning/<slug>` | `tuning/llm-timeout-300s` |
| 发布准备 | `release/<version>` | `release/0.2.0` |
| 文档/基建 | `chore/<slug>` · `docs/<slug>` | `chore/delete-tauri-app` |

### 2.2 创建与生命周期

```bash
# 始终基于最新 main 创建 worktree（禁止在主 checkout 切分支干活）
git fetch origin
git worktree add ../rmb-desktop-<slug> -b <prefix>/<slug> origin/main
```

- 会话开始时向用户声明：`分支 = <prefix>/<slug>，worktree = ../rmb-desktop-<slug>`。
- 尽早 push 分支（`git push -u origin <branch>`），让其他 agent 能看见，避免撞车。
- 完成时输出一份**交接报告**（做了什么 / 测试结果 / 未完成项 / 下一步），并记录到 rmb（`~/.rmb/bin/rmb`）。

### 2.3 工作树卫生（硬性）

- **离开 worktree 前必须清空工作树**：改动要么 commit，要么 `git stash`。脏文件（尤其二进制 icon、config 里的版本号）会跨分支"漂移"，是本次乱象的主要来源。
- 禁止把与任务无关的文件塞进 commit（例如：测试期间改的版本号、重新生成的 icns）。
- `dist/`、`bin/`、`app/node_modules`、`app/src-tauri/target` 已在 `.gitignore` 范围外的构建产物一律不提交。

### 2.4 合并流程（集成会话执行）

1. 合入前先 `git fetch origin`，把 main 合进特性分支并解决冲突；`go vet` + `make test` 全绿。
2. 合并顺序：若分支 A 依赖未合并分支 B，先合 B 再合 A。
3. 用 merge commit 合入 main，提交信息写 `Merge <branch>: <一句话>（agent session <id>）`。
4. 合并后删本地分支 + 远端分支（`git push origin --delete <branch>`），worktree `git worktree remove`。
5. 只有 main 上的 commit 才能进 release。

### 2.5 路径所有权（防冲突速查）

| 目录/文件 | 归属 |
|---|---|
| `internal/update/`、`cmd/manifest-sign/`、`scripts/build-sidecar-bundles.sh` | 自更新（updater）专享 |
| `internal/appshell/`、`cmd/rmb-app/`、`scripts/build-macos-app.sh`、`scripts/build-dmg.sh` | Go 壳（shell）专享 |
| `internal/hook/`、`internal/setup/` | agent 集成（cursor/claude/codex/opencode/pi/workbuddy）专享 |
| `webui/src/integrations/` | 对应集成的 UI，互不交叉 |
| `internal/pipeline`、`internal/worker/`、`internal/db/` | 数据管线，改动需 `make test` 全绿 + 说明行为不变 |
| `Makefile`、`scripts/release.sh`、`internal/version/` | 版本/发布相关，改动必须同步本文档 3 节 |
| `app/`（Tauri） | 待删除（Phase 4），除删除任务外禁止任何修改 |

> 若两个并行任务需要触碰同一目录：后启动的任务先把该目录状态告知用户，或排队等前一个合并。

---

## 3. 版本规范

### 3.1 语义化版本（SemVer）

格式 `MAJOR.MINOR.PATCH[-prerelease]`，规则：

- **MAJOR**：破坏性变更（配置格式、数据迁移、API 不兼容）。
- **MINOR**：新功能 / 壳层变更（例：shell 迁移 + 自更新器 = `0.2.0`）。
- **PATCH**：bug 修复、行为修正（例：`0.1.21 → 0.1.22`）。
- **prerelease**：仅用于暂存 feed 的 staging 构建（`0.2.0-rc.1`），**updater 永不自动安装 prerelease**（`internal/update` 已有该保护）。

### 3.2 单一版本来源（Single Source of Truth）

> 现状问题：`Makefile=0.1.22`、`tauri.conf.json=0.1.22`（未提交）、`app/package.json=0.1.21`、`internal/version/version.go=0.1.20` 四处不一致。

- **`Makefile` 顶部的 `VERSION ?=` 是唯一真源**。构建时通过 ldflags 注入 `internal/version.Version`，其他文件一律不作为版本源。
- `internal/version/version.go` 的默认值只是"无 ldflags 时的兜底"，仅用于 `go run`，发布构建必须带 ldflags；改版本时如顺手可同步，但不强求。
- **Tauri 遗留版本文件（`tauri.conf.json`、`app/package.json`）自即日起禁止再修改**，Phase 4 删除 `app/` 后彻底消失。

### 3.3 版本推进规则

- 只在 main 上推进版本：先合并全部待发功能 → 在 main 上 `git tag vX.Y.Z` → 构建发布。
- 同一版本号不得重复发布两次（发布失败重发需升 PATCH 或使用 `-rc.N` 重新构建）。
- 每发布一个版本，`internal/update` 的版本测试样例要补一条"当前版本 < 新版本"的用例。

### 3.4 发布流程（0.2.0 起，取代 Tauri 时代流程）

```bash
# 1. main 上打好 tag
git checkout main && git pull && git tag v0.2.0 && git push origin v0.2.0

# 2. 构建 + 签名 + 上传（Makefile 已接入 sidecar bundles + 签名 manifest）
make release VERSION=0.2.0          # DMG + sidecar bundles + 签名 manifest.json → GitHub
PUBLISH_R2=1 make release VERSION=0.2.0   # 同时推 releases.re-mem-ber.me（中国主 feed）

# 3. 验证 feed
curl https://releases.re-mem-ber.me/latest.json   # 必须等于 v0.2.0，且签名可验
```

- **每次发布必须重建 feed 全量**（`latest.json` + `versions.json` + 签名），修复 plan 中记录的"R2 `latest.json` 仍指向 0.1.0"的腐烂问题。
- `.cursor/rules/release.mdc` 中"Tauri 构建 + notarize"的旧描述已过时，发布前先更新该文件（见"待办"）。

### 3.5 版本规划（当前）

| 版本 | 状态 | 内容 |
|---|---|---|
| `0.1.21` | 已发布（GitHub latest） | Tauri 时代最后一个版本 |
| `0.1.22` | ❌ 取消作为公开版本 | 仅用于本地 E2E 测试 updater 的构建号（dist/ 里的产物），不要发到 GitHub |
| `0.2.0` | 下一个公开版本（计划） | Go 壳 + 自更新器（Phase 1-3 已合入 main 的部分）+ Phase 2 updater + WorkBuddy 集成 |
| `v0.1.21-tauri-final` | 待打 tag | Phase 5 要求：Tauri 版本的回滚锚点，从当前 main 前身打 |

---

## 4. 多 agent 并行铁律（操作层面）

1. **启动新 agent 前**，先 `git fetch origin` + 看 `git branch -r`，在任务描述里写明"基于 origin/main"还是"基于某个未合并分支"。
2. **禁止两个 agent 同时操作同一个 worktree 或主 checkout**；一个会话一个 pane，必要时用 herdr 管理。
3. **任何 agent 不得在未被告知的情况下：切分支、合并、push 到 main、打 tag、发 release**。
4. 会话结束必须：提交并清空工作树 → push 分支 → 写交接报告。
5. 集成会话（负责合并/发布）单独开一个会话执行，其他 agent 期间保持只读。

---

## 5. 当前待办清单（2026-08-15 快照，按依赖排序）

1. **决策**：0.2.0 是否包含 WorkBuddy 集成（若含，先合并 phase2 再合并 workbuddy，见 §2.4）。
2. 合并 `phase2-self-updater` → main（含 updater 核心 + 发布侧）。
3. 合并 `feat/workbuddy-integration` → main（依赖 2 的 6aad673，已包含，顺序无关）。
4. 重新构建并安装 `~/.rmb/bin/{rmb,rmbd-desktop,rmb-app}`（当前 20:57 的二进制不含 updater 与 workbuddy）。
5. Phase 4：`git rm -r app/`（清 4.4GB target）+ 清理 Makefile 残留目标。
6. 更新 `plan/tauri-to-go-shell.md` 状态行（Phase 2 已完成）与 `.cursor/rules/release.mdc`（新发布流程）。
7. Phase 5：0.2.0 发布矩阵、登录项自启验证（机器重启后未自动拉起，需确认）、打 `v0.1.21-tauri-final` tag。
8. 清理：主 checkout 未提交的 `icon.icns` / `tauri.conf.json` 改动（要么丢弃要么按 2.3 收尾）。
9. 核实 Cursor `rmb setup status` 显示 hook=none 的假阴性（hooks.json 用的是 `rmb-hook-dual` wrapper，检测不识别）——非阻塞，可顺带修。
