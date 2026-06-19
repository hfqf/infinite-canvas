#!/usr/bin/env python3
import html
import http.server
import math
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.parse
from pathlib import Path


ROOT = Path("/tmp/logo-vtracer-debug-service")
DEFAULT_INPUT = Path("/Users/points/Downloads/canvas-image-A_gdYMrNWORl0Y4zVOjOA.png")
VTRACER = Path("/Users/points/.cargo/bin/vtracer")
MAGICK = shutil.which("magick") or "/opt/homebrew/bin/magick"


def number(params, key, default, cast=float, min_value=None, max_value=None):
    raw = params.get(key, [str(default)])[0]
    try:
        value = cast(raw)
    except ValueError:
        value = default
    if min_value is not None:
        value = max(min_value, value)
    if max_value is not None:
        value = min(max_value, value)
    return value


def run_cmd(args):
    started = time.time()
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    elapsed = time.time() - started
    if result.returncode != 0:
        raise RuntimeError("\n".join([
            "command failed:",
            " ".join(str(x) for x in args),
            result.stdout.strip(),
            result.stderr.strip(),
        ]))
    return elapsed, result.stdout.strip(), result.stderr.strip()


def safe_name(value):
    return re.sub(r"[^a-zA-Z0-9_.-]+", "-", value).strip("-") or "run"


def analyze_svg(path):
    text = path.read_text(errors="ignore")
    paths = text.count("<path")
    fills = len(set(re.findall(r'fill="#[0-9A-Fa-f]{6}"', text)))
    return paths, fills, path.stat().st_size


def luma(color):
    return 0.2126 * color["r"] + 0.7152 * color["g"] + 0.0722 * color["b"]


def color_distance(a, b):
    return math.sqrt((a["r"] - b["r"]) ** 2 + (a["g"] - b["g"]) ** 2 + (a["b"] - b["b"]) ** 2)


def is_background_color(color):
    channels = [color["r"], color["g"], color["b"]]
    return (
        color["r"] >= 248 and color["g"] >= 248 and color["b"] >= 248
    ) or (luma(color) >= 235 and max(channels) - min(channels) <= 35)


def read_palette(path):
    _, stdout, _ = run_cmd([MAGICK, str(path), "-format", "%c", "-depth", "8", "histogram:info:-"])
    palette = []
    pattern = re.compile(r"^\s*(\d+):\s+\((\d+),(\d+),(\d+)")
    for line in stdout.splitlines():
        match = pattern.search(line)
        if not match:
            continue
        count = int(match.group(1))
        r = int(match.group(2))
        g = int(match.group(3))
        b = int(match.group(4))
        palette.append({"count": count, "r": r, "g": g, "b": b, "hex": f"#{r:02X}{g:02X}{b:02X}"})
    return sorted(palette, key=lambda item: item["count"], reverse=True)


def build_layers(palette):
    layers = []
    for color in palette:
        if is_background_color(color):
            continue
        nearest_index = -1
        nearest_distance = 999
        for index, layer in enumerate(layers):
            distance = color_distance(color, layer)
            if distance < nearest_distance:
                nearest_distance = distance
                nearest_index = index
        if nearest_index >= 0 and nearest_distance <= 96:
            layers[nearest_index]["colors"].append(color)
            layers[nearest_index]["count"] += color["count"]
            continue
        layers.append({**color, "colors": [color]})
    return sorted(layers, key=luma, reverse=True)


def svg_paths(path, fill):
    text = path.read_text(errors="ignore")
    paths = re.findall(r"<path\b[^>]*>", text, flags=re.S)
    return "\n".join(re.sub(r'fill="#[0-9A-Fa-f]{6}"', f'fill="{fill}"', item) for item in paths)


def generate_layered(params):
    input_path = Path(params.get("input", [str(DEFAULT_INPUT)])[0]).expanduser()
    if not input_path.exists():
        raise RuntimeError(f"input not found: {input_path}")
    ROOT.mkdir(parents=True, exist_ok=True)

    colors = number(params, "colors", 12, int, 2, 24)
    fuzz = number(params, "fuzz", 3, float, 0, 20)
    filter_speckle = number(params, "filter_speckle", 16, int, 0, 16)
    corner_threshold = number(params, "corner_threshold", 80, int, 0, 180)
    segment_length = number(params, "segment_length", 8, float, 3.5, 10)
    splice_threshold = number(params, "splice_threshold", 75, int, 0, 180)
    path_precision = number(params, "path_precision", 3, int, 0, 8)

    name = safe_name(f"layered-c{colors}-f{fuzz}-seg{segment_length}-ct{corner_threshold}-sp{splice_threshold}-fs{filter_speckle}")
    run_dir = ROOT / name
    run_dir.mkdir(parents=True, exist_ok=True)
    prep = run_dir / "prep.png"
    svg = run_dir / "out.svg"
    preview = run_dir / "preview.png"
    crop = run_dir / "crop-years.png"
    text_crop = run_dir / "crop-bottom-text.png"
    thirty_crop = run_dir / "crop-30-years.png"
    bls_crop = run_dir / "crop-bls-wave.png"
    main_crop = run_dir / "crop-main-logo.png"
    full = run_dir / "full.png"

    magick_args = [
        MAGICK, str(input_path),
        "-auto-orient", "-background", "white", "-alpha", "remove", "-alpha", "off",
        "-filter", "Lanczos", "-resize", "400%", "-resize", "4096x4096>",
        "-colorspace", "sRGB", "-fuzz", f"{fuzz}%", "-fill", "white", "-opaque", "white",
        "-colors", str(colors), "-dither", "none", "-strip", str(prep),
    ]
    magick_time, _, magick_err = run_cmd(magick_args)
    palette = read_palette(prep)
    layers = build_layers(palette)
    if not layers:
        raise RuntimeError("no traceable color layers")

    rendered_paths = []
    vtracer_time = 0
    marker = "#010203"
    for index, layer in enumerate(layers):
        mask = run_dir / f"layer-{index}.png"
        remove_mask = run_dir / f"layer-{index}-exclude.png"
        layer_svg = run_dir / f"layer-{index}.svg"
        mask_args = [MAGICK, str(prep)]
        for color in layer["colors"]:
            mask_args += ["-fill", marker, "-opaque", color["hex"]]
        mask_args += ["-fill", "white", "+opaque", marker, "-fill", "black", "-opaque", marker, "-type", "bilevel", "-strip", str(mask)]
        run_cmd(mask_args)
        exclude_colors = []
        for darker_layer in layers[index + 1:]:
            exclude_colors += darker_layer["colors"]
        if exclude_colors:
            exclude_args = [MAGICK, str(prep)]
            for color in exclude_colors:
                exclude_args += ["-fill", marker, "-opaque", color["hex"]]
            exclude_args += [
                "-fill", "black", "+opaque", marker,
                "-fill", "white", "-opaque", marker,
                "-type", "bilevel", "-morphology", "Dilate", "Disk:2",
                "-strip", str(remove_mask),
            ]
            run_cmd(exclude_args)
            run_cmd([MAGICK, str(mask), str(remove_mask), "-compose", "Lighten", "-composite", "-type", "bilevel", str(mask)])
        run_cmd([MAGICK, str(mask), "-blur", "0x0.8", "-threshold", "50%", "-type", "bilevel", str(mask)])
        elapsed, _, _ = run_cmd([
            str(VTRACER), "--input", str(mask), "--output", str(layer_svg), "--preset", "bw",
            "--mode", "spline", "--colormode", "bw", "--filter_speckle", str(filter_speckle),
            "--corner_threshold", str(corner_threshold), "--segment_length", str(segment_length),
            "--splice_threshold", str(splice_threshold), "--path_precision", str(path_precision),
        ])
        vtracer_time += elapsed
        rendered_paths.append(f'<g fill="{layer["hex"]}">\n{svg_paths(layer_svg, layer["hex"])}\n</g>')

    width_height = subprocess.check_output([MAGICK, "identify", "-format", "%w %h", str(prep)], text=True).strip()
    width, height = [int(part) for part in width_height.split()]
    svg.write_text(
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        f'<svg version="1.1" xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">\n'
        f'<rect width="{width}" height="{height}" fill="#FFFFFF"/>\n'
        + "\n".join(rendered_paths)
        + "\n</svg>\n"
    )
    render_time, _, _ = run_cmd([MAGICK, "-background", "white", str(svg), "-resize", "1200x", str(preview)])
    run_cmd([MAGICK, "-background", "white", str(svg), str(full)])
    run_cmd([MAGICK, str(full), "-crop", "1100x700+1450+850", "+repage", "-resize", "1600x", str(crop)])
    run_cmd([MAGICK, str(full), "-crop", "3300x500+500+1700", "+repage", "-resize", "1800x", str(text_crop)])
    run_cmd([MAGICK, str(full), "-crop", "1700x1350+250+150", "+repage", "-resize", "1600x", str(thirty_crop)])
    run_cmd([MAGICK, str(full), "-crop", "2200x1200+1500+800", "+repage", "-resize", "1600x", str(bls_crop)])
    run_cmd([MAGICK, str(full), "-crop", "3600x1550+250+550", "+repage", "-resize", "1800x", str(main_crop)])
    paths, fills, bytes_len = analyze_svg(svg)
    return {
        "name": name, "run_dir": run_dir, "prep": prep, "svg": svg, "preview": preview, "crop": crop,
        "text_crop": text_crop, "thirty_crop": thirty_crop, "bls_crop": bls_crop, "main_crop": main_crop,
        "paths": paths, "fills": fills, "bytes": bytes_len,
        "removed_text_shadows": 0, "magick_time": magick_time, "vtracer_time": vtracer_time,
        "render_time": render_time, "magick_err": magick_err,
        "vtracer_out": "layers=" + ", ".join(f'{layer["hex"]}:{len(layer["colors"])}' for layer in layers),
        "vtracer_err": "",
    }


def remove_text_shadow_paths(svg_path, min_y):
    removed = 0
    lines = []
    light_colors = {"90BAC4", "DAE8EB", "E6EFF2", "D2E5EA", "95C0D1"}
    pattern = re.compile(r'fill="#([0-9A-Fa-f]{6})".*transform="translate\(([-0-9.]+),([-0-9.]+)\)"')
    for line in svg_path.read_text().splitlines():
        match = pattern.search(line)
        if match:
            color = match.group(1).upper()
            y = float(match.group(3))
            if color in light_colors and y >= min_y:
                removed += 1
                continue
        line = re.sub(r'fill="#FEFEFE"', 'fill="#FFFFFF"', line)
        lines.append(line)
    svg_path.write_text("\n".join(lines) + "\n")
    return removed


def generate(params):
    if params.get("pipeline", ["layered"])[0] == "layered":
        return generate_layered(params)

    input_path = Path(params.get("input", [str(DEFAULT_INPUT)])[0]).expanduser()
    if not input_path.exists():
        raise RuntimeError(f"input not found: {input_path}")
    ROOT.mkdir(parents=True, exist_ok=True)

    colors = number(params, "colors", 10, int, 0, 64)
    posterize = number(params, "posterize", 0, int, 0, 64)
    blur = number(params, "blur", 0.8, float, 0, 4)
    fuzz = number(params, "fuzz", 4, float, 0, 20)
    median = number(params, "median", 2, int, 0, 9)
    color_precision = number(params, "color_precision", 4, int, 1, 8)
    gradient_step = number(params, "gradient_step", 24, int, 0, 255)
    corner_threshold = number(params, "corner_threshold", 70, int, 0, 180)
    segment_length = number(params, "segment_length", 6, float, 3.5, 10)
    splice_threshold = number(params, "splice_threshold", 60, int, 0, 180)
    filter_speckle = number(params, "filter_speckle", 16, int, 0, 16)
    path_precision = number(params, "path_precision", 3, int, 0, 8)
    remove_text_shadow = number(params, "remove_text_shadow", 1, int, 0, 1)
    text_shadow_min_y = number(params, "text_shadow_min_y", 1700, int, 0, 4096)

    name = safe_name(
        f"c{colors}-p{posterize}-b{blur}-f{fuzz}-m{median}-"
        f"cp{color_precision}-g{gradient_step}-seg{segment_length}-"
        f"ct{corner_threshold}-sp{splice_threshold}-fs{filter_speckle}-"
        f"ts{remove_text_shadow}-{text_shadow_min_y}"
    )
    run_dir = ROOT / name
    run_dir.mkdir(parents=True, exist_ok=True)
    prep = run_dir / "prep.png"
    svg = run_dir / "out.svg"
    preview = run_dir / "preview.png"
    crop = run_dir / "crop-years.png"
    text_crop = run_dir / "crop-bottom-text.png"

    magick_args = [
        MAGICK,
        str(input_path),
        "-auto-orient",
        "-background",
        "white",
        "-alpha",
        "remove",
        "-alpha",
        "off",
        "-filter",
        "Lanczos",
        "-resize",
        "400%",
        "-resize",
        "4096x4096>",
        "-colorspace",
        "sRGB",
        "-fuzz",
        f"{fuzz}%",
        "-fill",
        "white",
        "-opaque",
        "white",
    ]
    if blur > 0:
        magick_args += ["-blur", f"0x{blur}"]
    if colors > 0:
        magick_args += ["-colors", str(colors), "-dither", "none"]
    if posterize > 0:
        magick_args += ["-posterize", str(posterize)]
    if median > 0:
        magick_args += ["-statistic", "Median", str(median)]
    magick_args += ["-strip", str(prep)]
    magick_time, _, magick_err = run_cmd(magick_args)

    vtracer_args = [
        str(VTRACER),
        "--input",
        str(prep),
        "--output",
        str(svg),
        "--preset",
        "poster",
        "--mode",
        "spline",
        "--colormode",
        "color",
        "--hierarchical",
        "stacked",
        "--filter_speckle",
        str(filter_speckle),
        "--color_precision",
        str(color_precision),
        "--gradient_step",
        str(gradient_step),
        "--corner_threshold",
        str(corner_threshold),
        "--segment_length",
        str(segment_length),
        "--splice_threshold",
        str(splice_threshold),
        "--path_precision",
        str(path_precision),
    ]
    vtracer_time, vtracer_out, vtracer_err = run_cmd(vtracer_args)
    removed_text_shadows = 0
    if remove_text_shadow:
        removed_text_shadows = remove_text_shadow_paths(svg, text_shadow_min_y)

    render_time, _, _ = run_cmd([MAGICK, "-background", "white", str(svg), "-resize", "1200x", str(preview)])
    run_cmd([MAGICK, "-density", "144", "-background", "white", str(svg), str(run_dir / "full.png")])
    run_cmd([MAGICK, str(run_dir / "full.png"), "-crop", "1100x700+1450+850", "+repage", "-resize", "1600x", str(crop)])
    run_cmd([MAGICK, str(run_dir / "full.png"), "-crop", "5200x900+400+2400", "+repage", "-resize", "1800x", str(text_crop)])
    paths, fills, bytes_len = analyze_svg(svg)
    return {
        "name": name,
        "run_dir": run_dir,
        "prep": prep,
        "svg": svg,
        "preview": preview,
        "crop": crop,
        "text_crop": text_crop,
        "paths": paths,
        "fills": fills,
        "bytes": bytes_len,
        "removed_text_shadows": removed_text_shadows,
        "magick_time": magick_time,
        "vtracer_time": vtracer_time,
        "render_time": render_time,
        "magick_err": magick_err,
        "vtracer_out": vtracer_out,
        "vtracer_err": vtracer_err,
    }


def file_link(path, label=None):
    rel = path.relative_to(ROOT)
    href = "/file/" + urllib.parse.quote(str(rel))
    return f'<a href="{href}" target="_blank">{html.escape(label or path.name)}</a>'


def image_tag(path, title):
    rel = path.relative_to(ROOT)
    src = "/file/" + urllib.parse.quote(str(rel))
    return f"<h2>{html.escape(title)}</h2><img src='{src}' loading='lazy'>"


def form(params):
    def value(key, default):
        return html.escape(params.get(key, [str(default)])[0])

    fields = [
        ("pipeline", "layered", "text"),
        ("input", str(DEFAULT_INPUT), "text"),
        ("colors", 12, "number"),
        ("posterize", 0, "number"),
        ("blur", 0.8, "number"),
        ("fuzz", 3, "number"),
        ("median", 2, "number"),
        ("color_precision", 4, "number"),
        ("gradient_step", 24, "number"),
        ("corner_threshold", 80, "number"),
        ("segment_length", 8, "number"),
        ("splice_threshold", 75, "number"),
        ("filter_speckle", 16, "number"),
        ("path_precision", 3, "number"),
        ("remove_text_shadow", 1, "number"),
        ("text_shadow_min_y", 1700, "number"),
    ]
    rows = []
    for key, default, input_type in fields:
        step = "0.1" if key in {"blur", "fuzz", "segment_length"} else "1"
        rows.append(
            f"<label>{key}<input name='{key}' type='{input_type}' step='{step}' value='{value(key, default)}'></label>"
        )
    return "<form method='get'><input type='hidden' name='run' value='1'>" + "".join(rows) + "<button>Run</button></form>"


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path.startswith("/file/"):
            rel = urllib.parse.unquote(parsed.path[len("/file/"):])
            target = (ROOT / rel).resolve()
            if not str(target).startswith(str(ROOT.resolve())) or not target.exists():
                self.send_error(404)
                return
            self.send_response(200)
            if target.suffix == ".svg":
                self.send_header("Content-Type", "image/svg+xml")
            elif target.suffix == ".png":
                self.send_header("Content-Type", "image/png")
            else:
                self.send_header("Content-Type", "application/octet-stream")
            self.end_headers()
            self.wfile.write(target.read_bytes())
            return

        params = urllib.parse.parse_qs(parsed.query)
        body = [
            "<!doctype html><meta charset='utf-8'><title>VTracer Debug</title>",
            "<style>body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;margin:24px;color:#111}form{display:grid;grid-template-columns:repeat(4,minmax(160px,1fr));gap:12px;align-items:end}label{display:grid;gap:4px;font-size:12px}input{padding:7px;border:1px solid #ccc;border-radius:6px}button{padding:10px 14px;border:0;border-radius:6px;background:#111;color:white}img{max-width:100%;border:1px solid #ddd;background:white}pre{white-space:pre-wrap;background:#f6f6f6;padding:12px;border-radius:6px}</style>",
            "<h1>VTracer Logo Debug</h1>",
            form(params),
        ]
        if params.get("run", [""])[0] == "1":
            try:
                result = generate(params)
                body.append("<h2>Stats</h2>")
                body.append(
                    f"<p>paths={result['paths']} colors={result['fills']} bytes={result['bytes']} "
                    f"removedTextShadows={result['removed_text_shadows']} "
                    f"magick={result['magick_time']:.2f}s vtracer={result['vtracer_time']:.2f}s render={result['render_time']:.2f}s</p>"
                )
                body.append(
                    "<p>"
                    + " | ".join([
                        file_link(result["prep"], "prep.png"),
                        file_link(result["svg"], "out.svg"),
                        file_link(result["preview"], "preview.png"),
                        file_link(result["crop"], "crop-years.png"),
                        file_link(result["text_crop"], "crop-bottom-text.png"),
                        file_link(result.get("thirty_crop", result["crop"]), "crop-30-years.png"),
                        file_link(result.get("bls_crop", result["crop"]), "crop-bls-wave.png"),
                        file_link(result.get("main_crop", result["crop"]), "crop-main-logo.png"),
                    ])
                    + "</p>"
                )
                body.append(image_tag(result["prep"], "Preprocessed PNG"))
                body.append(image_tag(result["preview"], "SVG Preview"))
                if "main_crop" in result:
                    body.append(image_tag(result["main_crop"], "Main Logo Crop"))
                if "thirty_crop" in result:
                    body.append(image_tag(result["thirty_crop"], "30 Years Crop"))
                if "bls_crop" in result:
                    body.append(image_tag(result["bls_crop"], "BLS Wave Crop"))
                body.append(image_tag(result["text_crop"], "Bottom Text Crop"))
                body.append(image_tag(result["crop"], "Zoom Crop"))
                logs = "\n".join(x for x in [result["magick_err"], result["vtracer_out"], result["vtracer_err"]] if x)
                if logs:
                    body.append("<h2>Logs</h2><pre>" + html.escape(logs) + "</pre>")
            except Exception as exc:
                body.append("<h2>Error</h2><pre>" + html.escape(str(exc)) + "</pre>")

        data = "\n".join(body).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main():
    if not Path(MAGICK).exists():
        sys.exit("magick not found")
    if not VTRACER.exists():
        sys.exit("vtracer not found")
    ROOT.mkdir(parents=True, exist_ok=True)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 5177), Handler)
    print("VTracer debug server: http://127.0.0.1:5177/?run=1", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
