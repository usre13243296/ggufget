# -*- coding: utf-8 -*-
"""
从 ModelScope（魔搭）下载 GGUF 模型，并整理成 LM Studio 能识别的目录结构。

不碰 hosts、不改系统设置，纯 HTTP 下载。
实测同一文件：ModelScope ~8.5 MB/s，hf-mirror ~0.69 MB/s（国内）。

用法：
    # 列出仓库里所有 gguf
    python modelscope_download.py --repo lmstudio-community/Qwen2.5-7B-Instruct-GGUF --list

    # 下载指定量化
    python modelscope_download.py --repo lmstudio-community/Qwen2.5-7B-Instruct-GGUF --file Qwen2.5-7B-Instruct-Q4_K_M.gguf

    # 下载仓库里全部 gguf
    python modelscope_download.py --repo xxx/yyy --all

下载完成后会自动放到：
    %USERPROFILE%\\.lmstudio\\models\\<发布者>\\<模型名>\\<文件>.gguf
启动 LM Studio -> My Models 即可看到。
"""

import argparse
import json
import os
import re
import sys
import time
import urllib.parse
import urllib.request

API = "https://modelscope.cn/api/v1/models/{repo}/repo/files?Revision={rev}"
RESOLVE = "https://modelscope.cn/models/{repo}/resolve/{rev}/{path}"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) modelscope-download/1.0"


def lmstudio_models_dir() -> str:
    """读取 LM Studio 自己的设置，确定模型目录（默认 ~/.lmstudio/models）。"""
    settings = os.path.join(os.path.expanduser("~"), ".lmstudio", "settings.json")
    try:
        with open(settings, "r", encoding="utf-8-sig") as f:
            d = json.load(f)
        p = d.get("downloadsFolder")
        if p and os.path.isdir(p):
            return p
    except Exception:
        pass
    return os.path.join(os.path.expanduser("~"), ".lmstudio", "models")


def list_files(repo: str, rev: str = "master"):
    url = API.format(repo=repo, rev=rev)
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=30) as r:
        data = json.loads(r.read())
    files = (data.get("Data") or {}).get("Files") or []
    if not files:
        raise RuntimeError(f"仓库不存在或为空：{repo}（响应：{str(data)[:200]}）")
    return files


def human(n: float) -> str:
    for u in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}PB"


def download(url: str, dest: str, expect_size: int = 0):
    """断点续传下载，显示进度与速度。"""
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    have = os.path.getsize(dest) if os.path.exists(dest) else 0
    if expect_size and have == expect_size:
        print(f"[=] 已存在且大小一致，跳过：{os.path.basename(dest)}")
        return
    headers = {"User-Agent": UA}
    if have:
        headers["Range"] = f"bytes={have}-"
        print(f"[i] 续传，从 {human(have)} 开始")

    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=60) as r:
        total = expect_size or int(r.headers.get("Content-Length", 0) or 0)
        if have and r.status != 206:          # 服务器不支持 Range，只能重下
            have, total = 0, total
        mode = "ab" if have else "wb"
        t0, last, done = time.time(), 0.0, have
        with open(dest, mode) as f:
            while True:
                chunk = r.read(1024 * 1024)
                if not chunk:
                    break
                f.write(chunk)
                done += len(chunk)
                now = time.time()
                if now - last >= 1.0:
                    speed = (done - have) / max(now - t0, 1e-6)
                    pct = f"{done/total*100:5.1f}%" if total else "  ?  "
                    print(f"\r    {pct}  {human(done)} / {human(total) if total else '?'}  "
                          f"{human(speed)}/s   ", end="", flush=True)
                    last = now
    print()
    if expect_size and os.path.getsize(dest) != expect_size:
        raise RuntimeError(f"大小不符：期望 {expect_size}，实际 {os.path.getsize(dest)}")



def parse_selection(pool, req):
    """把 "2" / "1,3" / "1-3" 解析成 pool 里的下标；不是纯序号表达式则返回 None。"""
    q = req.strip().replace(" ", "")
    if not q:
        return None
    # 只有数字、逗号、短横线才当作序号表达式
    if not re.fullmatch(r"[0-9,\-]+", q):
        return None
    picks = []
    for part in q.split(","):
        if not part:
            continue
        m = re.fullmatch(r"(\d+)-(\d+)", part)
        if m:
            lo, hi = int(m.group(1)), int(m.group(2))
            if lo > hi:
                lo, hi = hi, lo
            picks.extend(range(lo, hi + 1))
        elif part.isdigit():
            picks.append(int(part))
        else:
            return None
    out = []
    for n in picks:
        if 1 <= n <= len(pool):
            out.append(pool[n - 1])
        else:
            return None
    return out if out else None


def match_files(pool, requests):
    """把用户给的 --file 参数匹配成仓库里的实际文件。

    支持三种写法：
      1. 列表序号：2 / 1,3 / 1-3 / "1 2"
      2. 关键词：q4_k_m（不区分大小写的包含匹配，唯一命中才算）
      3. 完整文件名或路径
    """
    chosen = []
    failed = False
    seen = set()
    for req in requests:
        q = req.strip().strip("'\"")
        # 容错：误把选项前缀打进输入框，例如 "--file xxx"、"file: xxx"
        for prefix in ("--file=", "--file ", "-f ", "file:", "文件名:", "文件名 "):
            if q.lower().startswith(prefix.rstrip(" ").lower()):
                q = q[len(prefix):].strip()
        q = q.replace("\\\\", "/")
        if not q:
            continue

        # 0) 序号选择
        by_index = parse_selection(pool, q)
        if by_index is not None:
            for s in by_index:
                if s.rfilename not in seen:
                    seen.add(s.rfilename)
                    chosen.append(s)
            picked = ", ".join(f"[{pool.index(s) + 1}] {os.path.basename(s.rfilename)}" for s in by_index)
            print(f"[=] 序号 {q} -> {picked}")
            continue

        # 1) 完整路径或文件名精确匹配
        hits = [s for s in pool if s.rfilename == q or os.path.basename(s.rfilename) == q]
        # 2) 不区分大小写精确
        if not hits:
            low = q.lower()
            hits = [s for s in pool
                    if s.rfilename.lower() == low
                    or os.path.basename(s.rfilename).lower() == low]
        # 3) 包含匹配（关键词）
        if not hits:
            low = q.lower()
            hits = [s for s in pool if low in s.rfilename.lower()]

        if not hits:
            print(f'[x] 没有匹配 "{req}" 的文件。')
            failed = True
            continue
        if len(hits) > 1:
            print(f'[!] "{req}" 匹配到 {len(hits)} 个文件，请写得更具体些：')
            for s in hits:
                print(f"      [{pool.index(s) + 1}] {s.rfilename}")
            failed = True
            continue
        s = hits[0]
        if s.rfilename not in seen:
            seen.add(s.rfilename)
            chosen.append(s)
            if os.path.basename(s.rfilename) != q:
                print(f'[=] "{req}" -> {s.rfilename}')

    if failed:
        print("\n提示：可以直接填列表序号（如 2，或 1,3），也可以填关键词（如 q4_k_m）。")
        print(f"      当前仓库共 {len(pool)} 个文件，用 --list 查看全部。")
        return [], True
    return chosen, False


def main():
    p = argparse.ArgumentParser(description="从 ModelScope 下载 GGUF 到 LM Studio")
    p.add_argument("--repo", required=True, help="如 lmstudio-community/Qwen2.5-7B-Instruct-GGUF")
    p.add_argument("--rev", default="master")
    p.add_argument("--file", action="append", default=[], help="要下载的文件名，可重复")
    p.add_argument("--all", action="store_true", help="下载全部 .gguf")
    p.add_argument("--list", action="store_true", help="只列出文件")
    p.add_argument("--any", action="store_true", help="--file 允许匹配非 gguf 文件")
    p.add_argument("--out", default=None, help="自定义输出目录（默认自动放进 LM Studio models）")
    a = p.parse_args()

    try:
        files = list_files(a.repo, a.rev)
    except Exception as e:
        print(f"[x] 获取文件列表失败：{e}")
        print("    检查仓库名是否正确，例如 lmstudio-community/Qwen2.5-7B-Instruct-GGUF")
        return 2

    ggufs = [f for f in files if f["Path"].lower().endswith(".gguf")]
    if a.list or not (a.file or a.all):
        print(f"[{a.repo}] 共 {len(ggufs)} 个 gguf：")
        for i, f in enumerate(ggufs, 1):
            print(f"    [{i}] {f['Path']:<56} {human(f.get('Size', 0))}")
        print("\n用 --file <序号或关键词> 下载指定的（如 --file 2、--file q4_k_m），"
              "或 --all 下载全部。")
        return 0

    if a.all:
        chosen = ggufs
    else:
        pool = files if a.any else ggufs
        chosen, bad = match_files(pool, a.file)
        if bad:
            return 2

    # 输出目录：LM Studio 用 <publisher>/<repo>/<file>
    publisher, repo_name = a.repo.split("/", 1)
    base = a.out or os.path.join(lmstudio_models_dir(), publisher, repo_name)

    print(f"[i] 仓库   : {a.repo} @ {a.rev}")
    print(f"[i] 输出到 : {base}")
    print(f"[i] 共 {len(chosen)} 个文件，总计 {human(sum(f.get('Size', 0) for f in chosen))}\n")

    for i, f in enumerate(chosen, 1):
        path = f["Path"]
        url = RESOLVE.format(repo=a.repo, rev=a.rev, path=urllib.parse.quote(path))
        rel = path.replace("/", os.sep)
        dest = os.path.join(base, rel)
        print(f"[{i}/{len(chosen)}] {path}  ({human(f.get('Size', 0))})")
        t = time.time()
        try:
            download(url, dest, int(f.get("Size", 0) or 0))
            print(f"[+] 完成，用时 {time.time()-t:.0f}s -> {dest}\n")
        except Exception as e:
            print(f"[x] 失败：{type(e).__name__}: {e}")
            print(f"    已下载部分保留在 {dest}，重新运行可续传。\n")
            return 3

    print("[+] 全部完成。打开 LM Studio → My Models 即可加载。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
