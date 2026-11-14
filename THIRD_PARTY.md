# Third-party software

The window uses lxn/walk and lxn/win (BSD-3-Clause,
https://github.com/lxn/walk, https://github.com/lxn/win). Images are decoded
with golang.org/x/image (BSD-3-Clause). Dependency versions and module checksums
are pinned in go.mod and go.sum.

The release builds are compiled with go-legacy-win7 (BSD-3-Clause, a Go fork
patched for Windows 7, https://github.com/thongtech/go-legacy-win7).

Play library, sync, flashing and factory restore come from
github.com/FoloToy/ai-passport-desktop (MIT). Its THIRD_PARTY.md applies to that
code, in short:

- Serial ports: go.bug.st/serial (BSD-3-Clause) and golang.org/x/sys
  (BSD-3-Clause).
- `pkg/esp` speaks the ESP32 serial bootloader protocol documented by Espressif,
  following esptool (GPL-2.0-or-later); no esptool code is included.
- The ESP32-C3 flasher stub is from esp-flasher-stub v1.4.1, dual licensed
  Apache-2.0 or MIT (texts in the ThirdParty folder of the ZIP).
  Source: https://github.com/espressif/esp-flasher-stub
- The AI Passport factory firmware (AI-Passport-Factory-Default-v1.1.2) is
  FoloToy's own firmware, embedded so a device can be restored offline.

`internal/driver/ai-passport-usb-serial.inf` only binds Windows' own usbser.sys
to the AI Passport serial interface.
`drivers/windows7/AI-Passport-Win7-JTAG-driver-Espressif-2021-07-15.zip` is
Espressif's signed WinUSB driver package for the JTAG interface, generated with
libwdi (LGPL-3.0); it is redistributed unchanged.

Play content, images and firmware are downloaded at runtime from the AI Passport
community and are not part of this repository or its ZIPs.
