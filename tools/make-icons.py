#!/usr/bin/env python3
"""Builds every app icon from one square master PNG (transparent background).

Usage: python tools/make-icons.py path/to/master.png

Writes:
  build/appicon.png                          1024 px, used by `wails build`
  build/windows/icon.ico                     16–256 px, Windows exe/taskbar
  build/linux/icons/hicolor/NxN/apps/belmemories.png  Linux desktop/taskbar
  frontend/src/assets/logo.png               256 px, in-app logo
"""
import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
MARGIN = 0.03  # transparent padding around the artwork, fraction of the side
LINUX_SIZES = [16, 22, 24, 32, 48, 64, 128, 256, 512]
ICO_SIZES = [16, 20, 24, 32, 40, 48, 64, 128, 256]


def master(src: Path) -> Image.Image:
    im = Image.open(src).convert("RGBA")
    bbox = im.getchannel("A").getbbox()
    if not bbox:
        sys.exit(f"{src}: image is fully transparent")
    im = im.crop(bbox)
    side = round(max(im.size) * (1 + 2 * MARGIN))
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(im, ((side - im.width) // 2, (side - im.height) // 2))
    return canvas.resize((1024, 1024), Image.LANCZOS)


def save(im: Image.Image, size: int, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    im.resize((size, size), Image.LANCZOS).save(path, optimize=True)
    print(f"  {path.relative_to(ROOT)}")


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    m = master(Path(sys.argv[1]))
    save(m, 1024, ROOT / "build/appicon.png")
    for s in LINUX_SIZES:
        save(m, s, ROOT / f"build/linux/icons/hicolor/{s}x{s}/apps/belmemories.png")
    save(m, 256, ROOT / "frontend/src/assets/logo.png")
    ico = ROOT / "build/windows/icon.ico"
    m.save(ico, sizes=[(s, s) for s in ICO_SIZES])
    print(f"  {ico.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
