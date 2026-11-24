# ggufget

**把 GGUF 模型从国内镜像快速拉进 LM Studio。单文件、零依赖、秒开。**

[![Release](https://img.shields.io/github/v/release/usre13243296/ggufget?label=release)](https://github.com/usre13243296/ggufget/releases)
[![License](https://img.shields.io/github/license/usre13243296/ggufget)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/usre13243296/ggufget)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-2f81f7)](#安装)

`ggufget` 解决的是一件很小但很烦的事：LM Studio 内置的下载走它自家的云代理，从国内访问常常只有 5 KB/s，一个 7 GB 的模型要挂一整天；而 huggingface 直连、hf-mirror、ModelScope 的速度又差出几个数量级，手动换地址、校验文件、摆对目录，每一步都能出错。

所以有了这个工具：一条命令，按 `ModelScope → hf-mirror → huggingface.co` 的优先级**自动选一个真的有这个仓库的镜像**，分段并发下载，按 LM Studio 的目录约定 `<文件名>/lmstudio-ai/<文件>.gguf` 摆好，打开 LM Studio 就能在 My Models 里看到它。

```console
$ ggufget doctor
$ ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
$ ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f 2
```

> English readers: [MANUAL.en.md](MANUAL.en.md) is the full English manual. The interface itself speaks zh / en / fr / es / ru.

## 目录

| 上手 | 使用 | 深入 |
| --- | --- | --- |
| [它解决什么问题](#它解决什么问题) · [特性](#特性) · [安装](#安装) · [30 秒上手](#30-秒上手) | [命令](#命令) · [常用选项](#常用选项) · [下载到哪里](#下载到哪里) · [中断、续传与校验](#中断续传与校验) · [日志](#日志) · [退出码](#退出码) · [配置](#配置) | [它是怎么工作的](#它是怎么工作的) · [文档](#文档) · [开发与发布](#开发与发布) · [许可](#许可) |

## 它解决什么问题

| 方式 | 实测参考 | 说明 |
| --- | --- | --- |
| LM Studio 官方代理 | ~5 KB/s | 256 KB 分片要 48 秒；20 MB 的 Range 请求直接断流 |
| huggingface.co 直连 | 常不可达 | DNS 被污染时 A 记录指向无关网段 |
| hf-mirror.com | 0.13 – 3.4 MiB/s | 覆盖最全，速度随网络波动 |
| ModelScope | 最高 8.5 MB/s（约 8.1 MiB/s） | 快，但模型覆盖不全 |

同一台机器上的实测中，最快与最慢相差三个数量级（单机单次，非同时测量）。数字会变：**以你自己机器上的 `ggufget doctor` 为准**（1 MiB/s = 1.05 MB/s）。

`ggufget` 不去猜“哪个最快”，而是同时探测三个镜像，按 **ModelScope → hf-mirror → huggingface.co** 的优先级取第一个真的能列出文件的。速度排名由 `doctor` 串行实测后展示，不参与自动选择——同一台机器上，三者的快慢一天里能反过来。

## 特性

- **自动选镜像**：并发探测三个镜像，按优先级取第一个真的有仓库的；成本约一次往返。
- **分段并发下载**：文件 ≥ 64 MiB、服务端支持 Range 且连接数 ≥ 2 时切段（默认 4 段）；进度写进 `.part.seg` journal，断网、Ctrl-C、关机后重跑同一条命令就能续传。
- **续传不拼版本**：journal 记录总大小与 ETag，服务器上的文件变了就从头再来，绝不把两个版本拼成一个文件。
- **会自己让路**：429/503 按 `Retry-After` 退避并把连接数减半；连接 15 秒没有字节就重连该段，而不是假装还在下。
- **完成才改名**：数据先写 `.part`，大小（以及按策略要求的 SHA256）都通过才 rename 成 `.gguf`；中途失败不会留下一个“看起来完整”的坏模型。
- **校验分级**：`auto` 只在镜像给了 SHA256 且文件 ≤ 4 GiB、续传过或重试过时算哈希，大文件首次下载省掉几分钟全量哈希；想强制就 `--verify sha256`。
- **零运行环境**：`CGO_ENABLED=0` 静态编译，Windows / Linux / macOS × amd64 / arm64 各一个文件，不装 Python、不装 .NET、不需要管理员权限。
- **管道友好**：结果走 stdout，进度走 stderr；`--json` 是 NDJSON 事件流。
- **五语言 + 分层帮助**：中文 / English / Français / Español / Русский；`help` 是命令表，长篇说明书是外置的 `MANUAL.md`，不塞进二进制。
- **双击也能用**：不带参数进入交互界面，支持仓库历史的 TAB 补全、↑↓ 历史、`q` 退出；出错会停住等你看完，不闪退。

## 安装

### Windows（PowerShell 5.1 或 7）

```powershell
irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex
```

装到 `%LOCALAPPDATA%\Programs\ggufget`，校验 SHA256，自动加进用户 PATH，不需要管理员权限。

### Linux / macOS

```sh
curl -fsSL https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.sh | sh
```

root 或 `/usr/local/bin` 可写时装到 `/usr/local/bin`，否则装到 `~/.local/bin` 并提示你加进 PATH。

### 手动

从 [Releases](https://github.com/usre13243296/ggufget/releases) 下载对应平台的单文件，Linux/macOS 先 `chmod +x`，再丢进 PATH。资产名格式：`ggufget_1.2.1_windows_amd64.exe`、`ggufget_1.2.1_linux_arm64`……

### 从源码

需要 Go 1.24+，无第三方依赖：

```sh
git clone https://github.com/usre13243296/ggufget
cd ggufget
make build     # 产出 dist/ggufget（Windows 上不会自动加 .exe，可自行重命名）
make dist      # 交叉编译六个平台 + 生成 dist/SHA256SUMS
```

Windows 上没装 `make`、想直接产出六平台发行包时，下面这段等价于 `make dist`：

```powershell
$env:CGO_ENABLED = "0"
$v = "1.2.1"
foreach ($t in @("windows/amd64","windows/arm64","linux/amd64","linux/arm64","darwin/amd64","darwin/arm64")) {
    $os, $arch = $t -split "/"
    $env:GOOS = $os; $env:GOARCH = $arch
    $ext = if ($os -eq "windows") { ".exe" } else { "" }
    go build -trimpath -ldflags "-s -w" -o "dist/ggufget_${v}_${os}_${arch}$ext" ./cmd/ggufget
}
```

## 30 秒上手

```sh
ggufget                      # 交互式界面：下载 / 搜索 / 测速 / 设置 / 帮助 / 退出
ggufget doctor               # 环境自检 + 三个镜像在你网络下的实测排名
ggufget list owner/repo      # 列出可下载的文件（序号 / 大小 / 量化）
ggufget get  owner/repo -f 2 # 下载默认视图里的第 2 个文件
```

`list` 的输出自带序号：

```text
序号  文件                                    大小      量化
--------------------------------------------------------------
1     Qwen2.5-7B-Instruct-Q3_K_L.gguf        3.81 GiB  Q3_K_L
2     Qwen2.5-7B-Instruct-Q4_K_M.gguf        4.36 GiB  Q4_K_M
3     Qwen2.5-7B-Instruct-Q6_K.gguf          5.82 GiB  Q6_K
4     Qwen2.5-7B-Instruct-Q8_0.gguf          7.54 GiB  Q8_0
```

> **序号规则（重要）**：`-f` 的序号按**默认视图**解释——只算 `.gguf`、按文件名排序。`list` 的 `--any` / `--filter` / `--sort` / `--limit` 只改变显示，不改变 `-f` 的解释。如果你看的是过滤或排序后的列表，请改用关键词或完整文件名，别用序号；完整文件名仍可匹配任意文件。

```sh
ggufget get owner/repo -f 2             # 默认视图的序号
ggufget get owner/repo -f q4_k_m        # 关键词（唯一命中才算数，歧义会列出候选）
ggufget get owner/repo -f 1,3           # 多个
ggufget get owner/repo -f 1-3           # 区间
ggufget get owner/repo --all -j 2       # 全部，两个文件并行
ggufget get owner/repo -f 2 --dry-run   # 只看计划，不下载
```

仓库地址怎么写都行，程序会自己清理：

```text
owner/repo
https://huggingface.co/owner/repo
https://hf-mirror.com/owner/repo/tree/main
https://www.modelscope.cn/models/owner/repo
「owner/repo」
```

## 命令

| 命令 | 作用 |
| --- | --- |
| `list <仓库>` | 列出文件；`--any` 连非 gguf 一起列，`--filter`、`--sort`、`--limit` 只影响显示 |
| `get <仓库>` | 下载；`-f` 选文件，`-a` 全下，`--dry-run` 只出计划 |
| `info <仓库>` | commit、许可、是否 gated、总大小与量化分布 |
| `search <关键词>` | 搜索仓库（只走 Hugging Face 搜索接口） |
| `doctor` | 环境自检 + 镜像测速排名 |
| `config` | 显示当前生效的配置；`--init` 写出默认配置，`--json` 给脚本用 |
| `version` / `help,h [命令]` | 版本 / 帮助总览、单个命令的完整帮助 |

## 常用选项

| 开关 | 说明 |
| --- | --- |
| `-f, --file <值>` | 默认视图序号 `2`、列表 `1,3`、区间 `1-3`、关键词 `q4_k_m`、文件名，可重复 |
| `-a, --all` | 下载全部 gguf |
| `-c, --connections <值>` | 单文件分段连接数，`0` = 自动（默认 4；文件 < 64 MiB 或服务端不支持 Range 时为 1） |
| `--verify <auto\|size\|sha256\|off>` | 校验策略，默认 `auto` |
| `-o, --out <目录>` | 输出目录，默认 LM Studio 模型目录 |
| `--layout <lmstudio\|repo\|flat>` | 目录布局，默认 `lmstudio`（见下一节） |
| `--provider <auto\|modelscope\|hf-mirror\|hf>` | 钉死某个镜像 |
| `--proxy <URL>` | 走代理（也认 `HTTPS_PROXY` / `HTTP_PROXY`） |
| `--json` | NDJSON 事件流，每行一个 JSON |
| `--lang <zh\|en\|fr\|es\|ru>` | 界面语言 |

完整选项：`ggufget help get`。长文说明书：[MANUAL.md](MANUAL.md)。

## 下载到哪里

默认是 LM Studio 的模型目录，按顺序找：

1. `--out`
2. 环境变量 `GGUFGET_MODELS_DIR`
3. 配置文件里的 `models_dir`
4. LM Studio `settings.json` 的 `downloadsFolder`（三平台都在 `~/.lmstudio/settings.json`）
5. `~/.lmstudio/models`（不存在会在下载时创建）

默认布局是 LM Studio 的约定：**第一层目录 = 文件名去掉 `.gguf`，第二层固定 `lmstudio-ai`**。比如下载 `DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf`：

```text
<模型目录>/DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/
└── lmstudio-ai/
    └── DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf
```

这两层目录会自动建好。想换别的摆法用 `--layout`：

| 值 | 落到 |
| --- | --- |
| `lmstudio`（默认） | `<文件名去扩展名>/lmstudio-ai/<文件>.gguf` |
| `repo` | `<发布者>/<仓库>/<文件>.gguf` |
| `flat` | 直接放进目标目录，不建子目录 |

`--no-lmstudio` 等价于 `--layout flat`。布局默认值也可以写进配置文件的 `layout` 字段，或用 `GGUFGET_LAYOUT` 覆盖。

下载完还会调 `lms ls --json` 确认模型真的被 LM Studio 索引到了（没装 `lms` 就跳过并给一句提示）。

## 中断、续传与校验

- 数据写在 `<目标>.part`，分段进度写在 `<目标>.part.seg`。
- Ctrl-C 或断网后，**重跑同一条命令**即可续传：单段模式按文件大小续，分段模式按 journal 里每段已完成的位置续。
- 完成后才改名成正式文件；中途失败不会留下一个看似完整的坏文件。
- 连接 15 秒没有新字节会自动重连该段；429/503 会退避并把连接数减半。
- `auto` 校验：文件 ≤ 4 GiB，或者这次下载续传/重试过，才核对 SHA256；大文件首次下载跳过全量哈希。想每次都校验用 `--verify sha256`，想最快用 `--verify off`。
- 哈希不符会删掉 `.part` 并返回退出码 4，下一次不会在坏数据上续传。

## 日志

日志默认开着，写在程序旁边（`ggufget.log`）；出错时终端会多打一行 `日志：<路径>`。它记的是终端上看不到的东西：完整命令行（token 已打码）、配置生效值与模型目录的生效值、每个文件的落盘绝对路径、每次网络探测的 URL/状态码/ETag/是否支持分段、失败的完整错误链与 `errno`（`Access is denied` 是 `errno=5`）、校验结果与退出码。

```sh
ggufget get owner/repo -f 1 --log ./debug.log   # 换位置
ggufget get owner/repo -f 1 --log-level debug   # 更详细
ggufget get owner/repo -f 1 --no-log            # 这次不写
```

程序目录不可写时自动退到用户缓存目录，再退到临时目录；超过 4 MiB 轮转成 `ggufget.log.1`。`ggufget doctor` 会显示日志路径与“可写/写不进去”，并对模型目录做写入测试。

## 退出码

| 码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 其它错误 |
| 2 | 参数/用法错误 |
| 3 | 网络错误（重试耗尽、429、超时） |
| 4 | 校验失败 |
| 5 | 磁盘空间不足 |
| 6 | 仓库或文件不存在 |
| 7 | 权限被拒或路径不可写 |
| 130 | 用户中断（Ctrl-C） |

脚本里判断 `$?` 即可区分“该清理磁盘”和“该换镜像”。

## 配置

`ggufget config` 显示当前生效值；`ggufget config --init` 把默认值写成文件（Windows：`%APPDATA%\ggufget\config.json`，其它：`~/.config/ggufget/config.json`；可用 `GGUFGET_CONFIG` 或 `--config` 指定别处）。

```json
{
  "provider": "auto",
  "hf_endpoint": "https://hf-mirror.com",
  "ms_endpoint": "https://modelscope.cn",
  "models_dir": "",
  "layout": "lmstudio",
  "connections": 4,
  "jobs": 1,
  "retries": 3,
  "timeout": 30,
  "verify": "auto",
  "lang": ""
}
```

`connections` / `jobs` 填 `0` 表示自动。优先级：**命令行 > 环境变量（`GGUFGET_*`、`HF_TOKEN`、`HTTPS_PROXY`）> 配置文件 > 默认值**。

默认配置位置写不进去时（安全软件拦截、只读配置目录），设置会改写到程序旁边的 `ggufget.config.json`，下次启动优先使用；用 `--config` 显式指定的路径写不进去则只报错，不再换位置。

## 它是怎么工作的

```text
argv / 交互菜单
      │
      ▼
  cli.Parse ──► config.Load（命令行 > 环境 > 文件 > 默认）
      │
      ▼
  app.exec ──► provider.Auto ──► ModelScope / hf-mirror / huggingface.co
      │                              │  文件列表 + commit + SHA256
      ▼                              ▼
  pick.Resolve（序号/关键词/文件名）  lmstudio.Rel（<stem>/lmstudio-ai/<file>）
      │                              │
      ▼                              ▼
  downloader：Range 探测 → 4 段并发 → .part.seg journal → 校验 → rename
      │
      ▼
  lms ls --json 可见性检查 · 日志（errno/错误链）· 退出码
```

代码分层：`internal/app` 负责编排，`cli` 解析参数，`provider` 抽象三个镜像，`downloader` 只管把字节可靠地落到磁盘，`lmstudio` 负责目录与 `lms`，`ui` / `termui` 负责终端，`i18n` 存全部文案。行为契约写在 [SPEC.md](SPEC.md)。

## 文档

- [docs/README.md](docs/README.md) — 文档索引
- [MANUAL.md](MANUAL.md) — 外置说明书（中文，仓库根目录）
- [MANUAL.en.md](MANUAL.en.md) — 同上，英文
- [SPEC.md](SPEC.md) — 命令行契约（行为的唯一来源）
- [docs/design-principles.md](docs/design-principles.md) — 一个好用的命令行工具应该具备什么
- [docs/verify.md](docs/verify.md) — 下载状态机与校验策略
- [docs/bench.md](docs/bench.md) — 三种镜像的测速方法与原始数据
- [docs/blog.md](docs/blog.md) — 写这个工具的来龙去脉（可作为博客正文）
- [docs/collaboration.md](docs/collaboration.md) — 协作与诊断协议
- [docs/legacy/](docs/legacy/) — 最早的两个 Python 脚本，留作对照

## 开发与发布

```sh
make test      # go test ./...
make check     # gofmt 检查 + go vet + 测试
make build     # 本机构建到 dist/
make dist      # 六平台交叉编译 + dist/SHA256SUMS
```

打标签即发布：`git tag v1.2.1 && git push origin v1.2.1` 会触发 GitHub Actions，自动跑测试、交叉编译六个平台、校验产物并创建 Release。安装脚本里的仓库地址默认是 `usre13243296/ggufget`；fork 后要发自己的版本，改 `install/install.ps1` 的 `-Repo` 和 `install/install.sh` 的 `REPO`。

本地发也可以：`make dist` 后把 `dist/` 里的六个二进制和 `SHA256SUMS` 一起上传到 Release。资产名必须保持 `ggufget_<版本>_<系统>_<架构>[.exe]`，安装脚本按这个规则拼 URL。

## 许可

MIT
