# -*- coding: utf-8 -*-
"""
用 huggingface_hub 从 hf-mirror 下载 GGUF，并整理成 LM Studio 的目录结构。

速度不如 ModelScope（实测同一文件 hf-mirror ~0.69 MB/s vs ModelScope ~8.5 MB/s），
但 HuggingFace 上的模型/量化选择更多，有些模型 ModelScope 上没有镜像。

首次使用需要安装依赖：
    python -m pip install -U huggingface_hub

用法：
    # 列出仓库里的 gguf
    python hf_download.py --repo lmstudio-community/Qwen2.5-7B-Instruct-GGUF --list

    # 下载指定量化
    python hf_download.py --repo author/repo --file xxx-Q4_K_M.gguf

    # 下载全部 gguf
    python hf_download.py --repo author/repo --all

    # 临时换别的镜像/官方站
    python hf_download.py --repo ... --endpoint https://huggingface.co

下载完成会自动整理到 %USERPROFILE%\\.lmstudio\\models\\<发布者>\\<模型名>\\
"""

import argparse
import json
import os
import re
import shutil
import sys
import time


def lmstudio_models_dir() -> str:
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


def human(n: float) -> str:
    for u in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}PB"


def ensure_hub(endpoint: str):
    # 必须在 import huggingface_hub 之前设置环境变量
    os.environ["HF_ENDPOINT"] = endpoint
    os.environ.setdefault("HF_HUB_DISABLE_TELEMETRY", "1")
    try:
        import huggingface_hub as hh
    except ImportError:
        print("[x] 缺少 huggingface_hub，正在安装...")
        import subprocess
        r = subprocess.run([sys.executable, "-m", "pip", "install", "-U", "huggingface_hub"],
                           capture_output=True, text=True)
        if r.returncode != 0:
            print(r.stdout[-1500:], r.stderr[-1500:])
            print("\n请手动执行： python -m pip install -U huggingface_hub")
            sys.exit(2)
        import huggingface_hub as hh
    print(f"[i] huggingface_hub {hh.__version__}  endpoint={endpoint}")
    return hh



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
    p = argparse.ArgumentParser(description="从 hf-mirror 下载 GGUF 到 LM Studio")
    p.add_argument("--repo", required=True)
    p.add_argument("--rev", default="main")
    p.add_argument("--file", action="append", default=[])
    p.add_argument("--all", action="store_true")
    p.add_argument("--list", action="store_true")
    p.add_argument("--endpoint", default="https://hf-mirror.com")
    p.add_argument("--token", default=None, help="私有仓库的 HF token（可选）")
    p.add_argument("--out", default=None)
    a = p.parse_args()

    hh = ensure_hub(a.endpoint)

    try:
        info = hh.HfApi(endpoint=a.endpoint, token=a.token).model_info(a.repo, revision=a.rev, files_metadata=True)
    except Exception as e:
        print(f"[x] 获取仓库信息失败：{type(e).__name__}: {e}")
        print(f"    当前 endpoint：{a.endpoint}")
        return 2

    siblings = [s for s in info.siblings if s.rfilename.lower().endswith(".gguf")]
    if a.list or not (a.file or a.all):
        print(f"[{a.repo}] 共 {len(siblings)} 个 gguf：")
        for i, s in enumerate(siblings, 1):
            print(f"    [{i}] {s.rfilename:<56} {human(s.size or 0)}")
        print("\n用 --file <序号或关键词> 下载指定的（如 --file 2、--file q4_k_m），"
              "或 --all 下载全部。")
        return 0

    if a.all:
        chosen = siblings
    else:
        chosen, bad = match_files(siblings, a.file)
        if bad:
            return 2

    publisher, repo_name = a.repo.split("/", 1)
    staging = os.path.join(os.path.expanduser("~"), ".cache", "hf_lmstudio_staging", publisher, repo_name)
    final = a.out or os.path.join(lmstudio_models_dir(), publisher, repo_name)

    print(f"[i] endpoint : {a.endpoint}")
    print(f"[i] 临时目录 : {staging}")
    print(f"[i] 最终目录 : {final}")
    print(f"[i] {len(chosen)} 个文件，共 {human(sum(s.size or 0 for s in chosen))}\n")

    # 用 hf_hub_download 拿（自带分块并发 + 断点续传 + 进度条）
    results = []
    for i, s in enumerate(chosen, 1):
        print(f"[{i}/{len(chosen)}] {s.rfilename}  ({human(s.size or 0)})")
        t = time.time()
        for attempt in range(1, 4):
            try:
                path = hh.hf_hub_download(a.repo, s.rfilename, revision=a.rev,
                                          local_dir=staging, token=a.token,
                                          endpoint=a.endpoint)
                results.append((s.rfilename, path))
                print(f"[+] 完成，用时 {time.time()-t:.0f}s\n")
                break
            except Exception as e:
                print(f"[!] 第 {attempt} 次失败：{type(e).__name__}: {e}")
                if attempt == 3:
                    print("[x] 放弃该文件。重新运行本命令会从断点继续。\n")
                    return 3
                time.sleep(2 * attempt)

    # 整理到 LM Studio 目录
    os.makedirs(final, exist_ok=True)
    for rfilename, path in results:
        dst = os.path.join(final, rfilename.replace("/", os.sep))
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        if os.path.abspath(path) == os.path.abspath(dst):
            continue
        if os.path.exists(dst) and os.path.getsize(dst) == os.path.getsize(path):
            print(f"[=] 已就位：{dst}")
            continue
        shutil.move(path, dst)
        print(f"[>] 已放入 LM Studio：{dst}")

    print("\n[+] 全部完成。打开 LM Studio → My Models 即可加载。")
    print(f"    模型目录：{final}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
