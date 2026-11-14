# 发版

只有用户要求时才发版。Release 页面只保留最新的一个版本。

## 1. 改版本号

| 位置 | 改什么 |
| --- | --- |
| `VERSION` | `X.Y.Z`（打包脚本通过 `-ldflags -X main.version=` 写进 exe） |
| `main.go` | `var version = "X.Y.Z-dev"` |
| `rsrc_windows_{386,amd64}.syso` | 用下面的命令重新生成 |

```sh
go run github.com/tc-hib/go-winres@v0.3.3 simply --icon assets/icon.png --manifest gui \
  --product-name "AI Passport 玩法社区 Windows 7 版" --file-description "AI Passport 玩法社区 Windows 7 版" --copyright FoloToy \
  --product-version X.Y.Z.0 --file-version X.Y.Z.0 --original-filename AIPassportPlays.exe --arch amd64,386
```

提交 `chore: release X.Y.Z` 并推到 `main`。

## 2. 打包

在 Mac 或 Linux 上（不需要 Windows）：

```sh
scripts/package.sh     # dist/AI-Passport-Desktop-Windows7-X.Y.Z-{x64,x86}.zip 和 .sha256
```

脚本会先跑 `go test ./...`，再用 go-legacy-win7 对两个架构 `vet` 和编译。拿 x64 版在 Windows 上跑一次 `AIPassportPlays.exe -self-test`（输出 `"passed":true`），有条件时用 `AIPP_CAPTURE` 看一眼界面（见 AGENTS.md）。

## 3. 发布

```sh
gh release create vX.Y.Z --target <完整的 commit SHA> --prerelease \
  --title "vX.Y.Z：一句话概括" --notes-file notes.md dist/*.zip dist/*.zip.sha256
```

- 没有在 Windows 7 真机上验收前，一律 `--prerelease`。
- 发布说明用中文：一句话概括 → 下载对照表（64 位 / 32 位）→ 新增 / 修复 → 测试情况（如实写在哪些系统上跑过、哪些没有，有没有代码签名）。

## 4. 清理旧版本

```sh
gh release delete vOLD --cleanup-tag --yes && git tag -d vOLD
```
