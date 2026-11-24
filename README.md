# ggufget

**把 GGUF 模型从国内镜像快速拉进 LM Studio。单文件、零依赖、秒开。**

`ggufget` 解决的是一件很小但很烦的事：LM Studio 内置的下载走的是它自家的云代理，从国内访问常常只有 5 KB/s，一个 7 GB 的模型要挂一整天。而 huggingface 直连、hf-mirror、ModelScope 的速度又不一样，手动换地址、校验文件、摆对目录，每一步都能出错。

所以有了这个工具：一条命令，自动挑能用的最快镜像，分段并发下载，按 LM Studio 的目录约定 `<文件名>/lmstudio-ai/<文件>.gguf` 摆好，打开 LM Studio 就能在 My Models 里看到它。

```
$ ggufget doctor
$ ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
$ ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f 2
```

> English readers: [MANUAL.en.md](MANUAL.en.md) has the full English manual. The interface itself speaks zh / en / fr / es / ru.

---

## 它到底解决了什么

| 方式 | 实测速度 | 说明 |
| --- | --- | --- |
| LM Studio 官方代理 | ~5 KB/s | 256 KB 分片要 48 秒；20 MB 的 Range 请求直接断流 |
| huggingface.co 直连 | 常常连不上 | DNS 被污染时 A 记录指向无关网段 |
| hf-mirror.com | 0.13 – 3.4 MB/s | 覆盖最全，速度随网络波动 |
| ModelScope | 最高 8.5 MB/s | 快，但模型覆盖不全 |

同一台机器、同一时间段实测，三者能差出两个数量级。`ggufget` 不去猜，而是**同时探测三个镜像**，按 ModelScope → hf-mirror → huggingface 的优先级选第一个真的有这个仓库的，然后把下载任务交给它。`ggufget doctor` 可以随时给你当前网络下的真实排名。

## 特性

- **自动选镜像**：并发探测，按实测优先级落子；探测结果缓存在同一次运行里，不重复握手。
- **分段并发下载**：大于 64 MiB 的文件自动切成 4 段并行拉，进度写进 `.part.seg` 日志；断网、按 Ctrl-C、关机之后重跑同一条命令就能续传。
- **会自己让路**：遇到 429/503 自动退避重试，并发数减半，而不是一路撞墙。
- **校验策略分级**：`auto` 只在小文件、续传过或重试过时才算 SHA256（大文件首次下载跳过全量哈希，省几分钟）；想强制就 `--verify sha256`。
- **没有运行环境**：`CGO_ENABLED=0` 静态编译，Windows / Linux / macOS × amd64 / arm64 六个平台各一个二进制，不装 Python、不装 .NET、不需要管理员权限。
- **结果与进度分离**：结果走 stdout，进度走 stderr，`ggufget list ... > files.txt` 和 `--json` 的 NDJSON 事件流都能进管道。
- **五种界面语言**：中文 / English / Français / Español / Русский，`--lang fr`、`GGUFGET_LANG=ru`，或在交互式界面的「设置」里切换，立即生效。
- **帮助是命令表，说明书是文件**：`ggufget help`（或菜单里按 `h`）列出全部命令和一句话说明；长篇说明书是外置的 `MANUAL.md` / `MANUAL.en.md`，不塞进二进制。
- **双击也能用**：不带参数进入交互式界面，启动就显示镜像和上次测到的速度，选完文件直接下载；支持 TAB 补全、↑↓ 翻历史、`q` 退出，双击 exe 不会闪退，出错会停住等你看完。

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

root 或 `/usr/local/bin` 可写时装到 `/usr/local/bin`，否则装到 `~/.local/bin` 并提示你把它加进 PATH。

### 手动

从 [Releases](https://github.com/usre13243296/ggufget/releases) 下载对应平台的单文件，给执行权限（Linux/macOS `chmod +x`）后丢进 PATH 即可。文件名格式：`ggufget_1.2.1_windows_amd64.exe`、`ggufget_1.2.1_linux_arm64`……

### 从源码

需要 Go 1.24+，无第三方依赖：

```sh
git clone https://github.com/usre13243296/ggufget
cd ggufget
make build     # 产出 dist/ggufget（Windows 上是 dist/ggufget.exe）
make dist      # 交叉编译六个平台 + 生成 dist/SHA256SUMS
```

Windows 上没装 `make` 也没关系，等价于：

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

## 发布

安装脚本里的仓库地址默认是 `usre13243296/ggufget`；如果你 fork 后要发自己的版本，改这两处：

- `install/install.ps1` 的 `-Repo`
- `install/install.sh` 的 `REPO`

然后打标签，GitHub Actions（`.github/workflows/release.yml`）会自动交叉编译六个平台、生成 `SHA256SUMS` 并挂到 Release 上：

```sh
git tag v1.2.1
git push origin v1.2.1
```

本地发也可以：`make dist` 之后把 `dist/` 里的六个二进制和 `SHA256SUMS` 一起上传到 Release。资产名必须保持 `ggufget_<版本>_<系统>_<架构>[.exe]`，安装脚本按这个规则拼 URL。

## 上手

```sh
ggufget                      # 交互式界面：下载 / 搜索 / 测速 / 设置 / 帮助 / 退出
ggufget doctor               # 环境自检 + 三个镜像的真实测速排名
ggufget list owner/repo      # 列出可下载的文件（序号 / 大小 / 量化）
ggufget get  owner/repo -f 2 # 下载第 2 个文件
```

`list` 的输出自带序号，选文件时序号、关键词、文件名都能用：

```
序号  文件                                    大小  量化
------------------------------------------------------------
1     Qwen2.5-7B-Instruct-Q3_K_L.gguf        3.81 GiB  Q3_K_L
2     Qwen2.5-7B-Instruct-Q4_K_M.gguf        4.36 GiB  Q4_K_M
3     Qwen2.5-7B-Instruct-Q6_K.gguf          5.82 GiB  Q6_K
4     Qwen2.5-7B-Instruct-Q8_0.gguf          7.54 GiB  Q8_0
```

```sh
ggufget get owner/repo -f 2         # 序号
ggufget get owner/repo -f q4_k_m    # 关键词（唯一命中才算数，歧义会列出候选）
ggufget get owner/repo -f 1,3       # 多个
ggufget get owner/repo --all -j 2   # 全部，两个文件并行
ggufget get owner/repo -f 2 --dry-run   # 只看计划，不下载
```

仓库地址怎么写都行，脚本会自己清理：

```
owner/repo
https://huggingface.co/owner/repo
https://hf-mirror.com/owner/repo/tree/main
https://www.modelscope.cn/models/owner/repo
「owner/repo」
```

## 命令

| 命令 | 作用 |
| --- | --- |
| `list <仓库>` | 列出文件；`--any` 连非 gguf 一起列，`--filter`、`--sort`、`--limit` 用于筛选 |
| `get <仓库>` | 下载；`-f` 选文件，`-a` 全下，`--dry-run` 只出计划 |
| `info <仓库>` | 默认分支、许可、是否 gated、总大小、哪些镜像有 |
| `search <关键词>` | 搜索仓库（走 Hugging Face 搜索接口） |
| `doctor` | 环境自检 + 镜像测速排名 |
| `config` | 显示当前生效的配置；`--init` 写出默认配置，`--json` 给脚本用 |
| `version` / `help,h [命令]` | 版本 / 帮助总览、单个命令的完整帮助 |

## 几个常用开关

| 开关 | 说明 |
| --- | --- |
| `-f, --file <值>` | 序号 `2`、列表 `1,3`、区间 `1-3`、关键词 `q4_k_m`、文件名，可重复 |
| `-a, --all` | 下载全部 gguf |
| `-c, --connections <值>` | 单文件分段连接数，`0` = 自动（默认 4，小文件自动降为 1） |
| `--verify <auto\|size\|sha256\|off>` | 校验策略，默认 `auto` |
| `-o, --out <目录>` | 输出目录，默认 LM Studio 模型目录 |
| `--layout <lmstudio\|repo\|flat>` | 目录布局，默认 `lmstudio`（见下一节） |
| `--provider <auto\|modelscope\|hf-mirror\|hf>` | 钉死某个镜像 |
| `--proxy <URL>` | 走代理（也认 `HTTPS_PROXY`） |
| `--json` | NDJSON 事件流，每行一个 JSON，方便脚本消费 |
| `--lang <zh\|en\|fr\|es\|ru>` | 界面语言 |

完整选项：`ggufget help get`。长篇说明书是外置文件：[MANUAL.md](MANUAL.md)（交互式界面里按 `h` 查看总览，并会告诉你说明书放在哪）。

## 下载到哪里

默认是 LM Studio 的模型目录，按 SPEC 的顺序找：

1. `--out`
2. 环境变量 `GGUFGET_MODELS_DIR`
3. 配置文件里的 `models_dir`
4. LM Studio `settings.json` 的 `downloadsFolder`（Windows 在 `%USERPROFILE%\.lmstudio\settings.json`）
5. `~/.lmstudio/models`

默认布局是 LM Studio 的约定：**第一层目录 = 文件名去掉 `.gguf`，第二层固定 `lmstudio-ai`**。比如下载 `DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf`：

```
<模型目录>/DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/
└── lmstudio-ai/
    └── DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf
```

这两层目录 `ggufget` 会自动建好。想换别的摆法用 `--layout`：

| 值 | 落到 |
| --- | --- |
| `lmstudio`（默认） | `<文件名去扩展名>/lmstudio-ai/<文件>.gguf` |
| `repo` | `<发布者>/<仓库>/<文件>.gguf` |
| `flat` | 直接放进目标目录，不建子目录 |

`--no-lmstudio` 等价于 `--layout flat`，适合只想把文件存到别处的情况。布局默认值也可以写进配置文件的 `layout` 字段，或用 `GGUFGET_LAYOUT` 环境变量覆盖。

下载完还会调 `lms ls --json` 确认模型真的被 LM Studio 索引到了（没装 `lms` 就跳过，并给一句提示）。

## 中断、续传与校验

- 下载中的数据写在 `<目标>.part`，分段进度写在 `<目标>.part.seg`。
- Ctrl-C 或断网后，**重跑同一条命令**即可续传：单段模式按文件大小续，分段模式按 journal 里每段已完成的位置续。
- 完成后才改名成正式文件，中途失败不会留下一个看似完整的坏文件。
- `auto` 校验：文件 ≤ 4 GiB，或者这次下载续传/重试过，才核对 SHA256；超大文件首次下载跳过全量哈希。想每次都校验就 `--verify sha256`，想最快就 `--verify off`。

## 日志

日志默认开着，就写在程序旁边（`ggufget.log`）；出错时终端会多打一行 `日志：<路径>`。它记的是终端上看不到的东西：完整命令行（token 已打码）、配置生效值与模型目录来源、每个文件的落盘绝对路径、每次网络探测的 URL/状态码/ETag/是否支持分段、失败的完整错误链与 `errno`（`Access is denied` 是 `errno=5`）、校验结果和退出码。

```sh
ggufget get owner/repo -f 1 --log ./debug.log   # 换位置
ggufget get owner/repo -f 1 --log-level debug   # 更详细
ggufget get owner/repo -f 1 --no-log            # 这次不写
```

程序目录不可写时自动退到用户缓存目录再到临时目录；超过 4 MiB 轮转成 `ggufget.log.1`。`ggufget doctor` 会显示日志路径与"可写/写不进去"，并对模型目录做写入测试。

## 退出码

| 码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 2 | 参数错误 |
| 3 | 网络错误 |
| 4 | 校验失败 |
| 5 | 磁盘空间不足 |
| 6 | 仓库/文件不存在 |
| 7 | 权限被拒 |
| 130 | 用户中断 |

## 配置

`ggufget config` 显示当前生效值；`ggufget config --init` 把默认值写成文件（Windows：`%APPDATA%\ggufget\config.json`，其它：`~/.config/ggufget/config.json`，可用 `GGUFGET_CONFIG` 或 `--config` 指定别处）。

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

命令行 > 环境变量（`GGUFGET_*`、`HF_TOKEN`、`HTTPS_PROXY`）> 配置文件 > 默认值。

## 文档

- [MANUAL.md](MANUAL.md) — 外置说明书（中文，随发行包一起提供）
- [MANUAL.en.md](MANUAL.en.md) — 同上，英文
- [docs/design-principles.md](docs/design-principles.md) — 一个好用的命令行工具应该具备什么
- [docs/blog.md](docs/blog.md) — 写这个工具的来龙去脉（可作为博客正文）
- [docs/bench.md](docs/bench.md) — 三种镜像的测速方法与原始数据
- [docs/verify.md](docs/verify.md) — 校验策略与 `.part` 状态机的说明
- [docs/legacy/](docs/legacy/) — 最早那两个 Python 脚本，留作对照

## 许可

MIT
