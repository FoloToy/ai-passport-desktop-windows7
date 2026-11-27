#!/bin/bash
# Build the native Windows 7 / 8 / 8.1 client (x64 and 32-bit ZIPs) from macOS or Linux.
# Current Go no longer runs on Windows 7, so the client is compiled with go-legacy-win7
# (Go 1.26 patched for Windows 7). The window is plain Win32 (lxn/walk); no browser.
# Env: PLAYS_DIST (output dir, default dist), ALLOW_DIRTY=1 (skip clean-tree check).
set -euo pipefail
cd "$(dirname "$0")/.."

# Pinned toolchain: https://github.com/thongtech/go-legacy-win7/releases/tag/v1.26.11-1
gw7_tag=v1.26.11-1
gw7_ver=1.26.11-1
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) gw7_os=darwin_arm64; gw7_sha=60996604cc7fe8c89f1ee218050def2c9bdf562ca860113efa6deb090394f463 ;;
  Darwin-x86_64) gw7_os=darwin_amd64; gw7_sha=315e82d5b77b4bb72dcd8d5c44fd8e20c7a6e73492a68fe09e80c0447df988f1 ;;
  Linux-x86_64) gw7_os=linux_amd64; gw7_sha=b3aaa2206a79a62d0552485d60240e512555cec701edd4fc0b5ff45644e0c21f ;;
  Linux-aarch64) gw7_os=linux_arm64; gw7_sha=88430c039eef696adb20111076c04bbb51286b93147b281e4d3073d87727bffe ;;
  *) echo "No pinned go-legacy-win7 toolchain for $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

version=$(tr -d '[:space:]' < VERSION)
dest=${PLAYS_DIST:-dist}
if [ "${ALLOW_DIRTY:-0}" != 1 ] && [ -n "$(git status --porcelain)" ]; then
  echo "Commit the source tree before packaging (or set ALLOW_DIRTY=1)." >&2; exit 1
fi

tool=.runtime/go-legacy-win7-$gw7_ver-$gw7_os
if [ ! -x "$tool/go-legacy-win7/bin/go" ]; then
  mkdir -p .runtime/downloads "$tool"
  archive=.runtime/downloads/go-legacy-win7-$gw7_ver.$gw7_os.tar.gz
  curl -fsSL -o "$archive" "https://github.com/thongtech/go-legacy-win7/releases/download/$gw7_tag/go-legacy-win7-$gw7_ver.$gw7_os.tar.gz"
  echo "$gw7_sha  $archive" | shasum -a 256 -c -
  tar -xzf "$archive" -C "$tool"
fi
go7=$(cd "$tool/go-legacy-win7/bin" && pwd)/go
"$go7" version

# Tests of the platform-independent packages run on the build machine.
GOTOOLCHAIN=local "$go7" test ./...

# The flasher stub licenses ship with the shared code (ai-passport-desktop/pkg/esp).
desktop=$(GOTOOLCHAIN=local "$go7" list -m -f '{{.Dir}}' github.com/FoloToy/ai-passport-desktop)

export GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows
rm -rf "$dest"
mkdir -p "$dest"
for arch in amd64 386; do
  label=x64; [ "$arch" = 386 ] && label=x86
  name="AI-Passport-$version-Windows7-$label"
  pkg="$dest/$name"
  mkdir -p "$pkg/drivers" "$pkg/ThirdParty/esp-flasher-stub"
  GOARCH=$arch "$go7" vet ./...
  GOARCH=$arch "$go7" build -trimpath -ldflags "-s -w -H windowsgui -X main.version=$version" -o "$pkg/AI Passport.exe" .
  cp internal/driver/ai-passport-usb-serial.inf "$pkg/drivers/"
  cp drivers/windows7/AI-Passport-Win7-JTAG-driver-Espressif-2021-07-15.zip "$pkg/drivers/"
  cp LICENSE THIRD_PARTY.md "$pkg/ThirdParty/"
  cp "$desktop/pkg/esp/stub/LICENSE-APACHE" "$desktop/pkg/esp/stub/LICENSE-MIT" "$pkg/ThirdParty/esp-flasher-stub/"
  git rev-parse HEAD > "$pkg/source-commit.txt" 2>/dev/null || true
  sed 's/$/\r/' > "$pkg/START-HERE.txt" <<TXT
AI Passport ${version} / Windows 7、8、8.1 版（${label}）

1. 把整个文件夹解压到电脑上（不要在压缩包里直接运行），双击 AI Passport.exe。
   不需要安装浏览器或其他软件。
2. 第一次打开时选择玩法存储目录，点"开始更新"，把玩法下载到电脑上。
3. 插上 AI Passport。Windows 7/8 第一次使用要装一次串口驱动（驱动已经包含在程序里）：
   程序发现设备缺驱动时会自动询问是否安装，点"是"，Windows 请求管理员权限时再点"是"；
   如果提示"无法验证此驱动程序软件的发布者"，选"始终安装此驱动程序软件"。装好后重新插拔设备。
   也可以随时在"设备工具"页点"安装驱动"。
   自动安装失败时，按 https://github.com/FoloToy/ai-passport-desktop-windows7 的说明用 Zadig 手动安装。
4. 在"玩法社区"页选择玩法，点"刷入设备"。

32 位 Windows 用 x86 版，64 位用 x64 版（"计算机"右键 → 属性 → 系统类型）。
本版本没有代码签名，杀毒软件可能误报。

Native Windows 7/8/8.1 client. Extract the folder and run "AI Passport.exe"; no browser
is needed. The serial driver is built in: when you plug in an AI Passport that has no
driver, the app offers to install it (or use "Install driver" on the Device tools page).
TXT
  (cd "$dest" && rm -f "$name.zip" && zip -qr "$name.zip" "$name" && shasum -a 256 "$name.zip" > "$name.zip.sha256")
done
printf 'Packaged the Windows 7 client in %s\n' "$dest"
ls -l "$dest"/*.zip
