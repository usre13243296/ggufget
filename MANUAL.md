# ggufget 使用说明书

ggufget 把 GGUF 模型从镜像站下载到 LM Studio 的模型目录，下完就能在
My Models 里看到它。它只有一个可执行文件，不需要 Python、不需要 pip、
不需要改 hosts。

## intro 它解决什么问题

LM Studio 默认通过它自己的云代理下载模型（`search.lmstudio.ai/v1/hf-proxy`）。
在国内这条链路非常慢 —— 实测每秒只有几 KB，一个 7 GB 的模型要下好几天，
而且大文件经常中断。

ggufget 换了一条路：

1. 直接问镜像站要文件列表，所以你能看见每个量化的体积；
2. 用多连接分段 + Range 断点续传下载，中断了再跑一次会接着下；
3. 下完按 LM Studio 的目录约定摆好：`<模型目录>/<文件名去扩展名>/lmstudio-ai/<文件>.gguf`。

它不碰 `hosts`，不装驱动，不改系统设置，所以安全软件不会拦它。

## install 安装

Windows（PowerShell 5.1 或 7 都行）：

```powershell
irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex
```

Linux / macOS：

```sh
curl -fsSL https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.sh | sh
```

安装器会做四件事：识别系统架构 → 下载对应单文件 → 校验 SHA256 →
放到 PATH 里。也可以用源码编译（需要 Go 1.24+）：

```sh
git clone https://github.com/usre13243296/ggufget
cd ggufget
make build
```

升级就重跑一次安装器（同版本会跳过，`-Force` 可强制重装）。

## shell 交互式界面

不带参数运行 `ggufget`（或双击 exe）就进入交互式界面。启动时先打印状态：

```
ggufget 1.2.1
模型目录  C:\Users\you\.lmstudio\models
镜像      modelscope 8.5 MiB/s  ·  hf-mirror 0.7 MiB/s
[1] 下载模型   [2] 搜索仓库   [3] 测速   [4] 设置   [h] 帮助   [q] 退出
```

速度是上次测速的结果；还没测过就显示「按 [3] 测一下」。

```
[1] 下载模型    输入仓库 → 列出文件 → 填序号/关键词 → 看计划 → 确认
[2] 搜索仓库    关键词 → 编号列表 → 选一个 → 同上
[3] 测速        串行测三个镜像，数字写回配置，下次启动直接显示
[4] 设置        语言 / 目录布局 / 镜像 / 模型目录
[h] 帮助        全部命令 + 一句话说明 + 全局选项 + 退出码
[q] 退出
```

**键盘**

| 键 | 作用 |
|---|---|
| `1`..`4`、`h`、`q` | 选菜单项；也可以直接输入选项文字（`下载模型` / `download`） |
| `TAB` | 补全：仓库历史、命令名、语言/布局/镜像的取值、当前层选项键 |
| `↑` `↓` | 翻输入历史（最多 200 条，存在配置里） |
| `←` `→` `Home` `End` | 行内移动光标 |
| `Backspace` `Delete` | 删除字符 |
| `Ctrl+A` / `Ctrl+E` | 跳到行首 / 行尾 |
| `Ctrl+K` / `Ctrl+U` / `Ctrl+W` | 删到行尾 / 删到行首 / 删一个词 |
| `Ctrl+C` | 中断当前下载，或回到上一层 |
| `q` | 退出当前层；在主菜单里就是退出程序 |

**设置**（菜单 `[4]`）改完立即生效并写回配置文件：

1. 界面语言：中文 / English / Français / Español / Русский（也可用 `--lang fr`、`GGUFGET_LANG=ru` 一次性指定）。
2. 目录布局：`lmstudio` / `repo` / `flat`，等价于 `--layout`。
3. 默认镜像：`auto` / `modelscope` / `hf-mirror` / `hf`，等价于 `--provider`。
4. 模型目录：留空就继续用 LM Studio 设置里的那个。

**帮助与说明书**

- 菜单里按 `h`，或命令行 `ggufget help`：全部命令 + 一句话说明 + 全局选项 + 退出码。
- `ggufget help get`：单个命令的全部选项和示例。
- 长篇说明就是本文档：程序会在当前目录、exe 同目录、`exe/docs` 里找 `MANUAL.md`，找到就把路径告诉你；**长文不编译进二进制**。
- 管道或重定向时界面自动退化成逐行提问，不进入 raw 模式。

## providers 镜像

| 名称 | 地址 | 说明 |
| --- | --- | --- |
| `modelscope` | modelscope.cn | 国内直连最快，实测约 8.5 MB/s |
| `hf-mirror` | hf-mirror.com | Hugging Face 的完整镜像，覆盖最全 |
| `hf` | huggingface.co | 官方源，需要代理 |

默认 `--provider auto`：三个源同时探测，谁先给出文件列表就用谁，顺序是
modelscope → hf-mirror → hf。想固定一个源：

```sh
ggufget get owner/repo -f 1 -p modelscope
```

把稳定的选择写进配置，就不用每次带参数：

```json
{ "provider": "modelscope" }
```

注意：ModelScope 只镜像了一部分模型。如果它上面没有这个仓库，auto 会自动
落到 hf-mirror。

## select 怎么选文件

先看清单：

```sh
ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
```

```
[modelscope] 共 4 个 gguf，合计 21.5 GiB（其余文件用 --any 查看）
序号  文件                                    大小        量化
----  --------------------------------------  ----------  --------
   1  Qwen2.5-7B-Instruct-Q3_K_L.gguf         3.8 GiB     Q3_K_L
   2  Qwen2.5-7B-Instruct-Q4_K_M.gguf         4.4 GiB     Q4_K_M
   3  Qwen2.5-7B-Instruct-Q6_K.gguf           5.8 GiB     Q6_K
   4  Qwen2.5-7B-Instruct-Q8_0.gguf           7.5 GiB     Q8_0
```

`-f` 接受四种写法，可以对同一个仓库重复用：

- 序号：`-f 2`、`-f 1,3`、`-f 1-3`
- 文件名片段：`-f q4_k`、`-f q6`
- 完整文件名：`-f Qwen2.5-7B-Instruct-Q4_K_M.gguf`
- 全下：`-a`

片段匹配不分大小写；一个片段命中多个文件时不会瞎猜，而是把候选列出来让你
写具体些。`ggufget list --filter q4 --sort size` 可以先缩小清单再选序号。

## download 下载

```sh
ggufget get owner/repo -f 2
```

会先打印一份计划（源、commit、落盘路径、每个文件多大），确认后开始。常用开关：

| 开关 | 作用 |
| --- | --- |
| `-y, --yes` | 不再确认，适合脚本 |
| `--dry-run` | 只打印计划，不下载 |
| `-o, --out` | 换一个输出目录 |
| `-j, --jobs` | 同时下几个文件（默认 1） |
| `-c, --connections` | 单个文件用几条连接（0 = 自动，默认 4） |
| `--layout` | 目录布局：`lmstudio`（默认）、`repo`、`flat` |
| `--no-lmstudio` | 等于 `--layout flat`，直接放进目标目录 |
| `--overwrite` | 覆盖已存在的同名文件 |

下载逻辑：

- 先发 `Range: bytes=0-0` 探一次，确认服务器支持分段和文件总大小；
- 大文件切成若干段并行下载，每 2 秒把进度写进 `<文件名>.part.seg`；
- 断网、超时、429 限速都会自动退避重试，重试过的文件一定会做 SHA256 校验；
- 全部写完才把 `.part` 改名成正式文件，所以你不会看到一个半截的 `.gguf`。

中途 Ctrl+C 是安全的：已经下好的部分留在 `.part` 里，再跑同一条命令会
接着下。

## verify 校验

```sh
ggufget get owner/repo -f 1 --verify sha256
```

`--verify` 四档：

- `auto`（默认）：小于 4 GiB 的文件，或者这次下载发生过续传/重试，才校验
  SHA256；镜像没给 hash 就只比大小。
- `size`：只比大小，最快。
- `sha256`：强制校验，镜像没给 hash 就报错退出（退出码 4）。
- `off`：什么都不查。

校验不通过时，坏文件会被删掉，然后你可以直接重跑。想手工指定期望值：

```sh
ggufget get owner/repo -f 1 --sha256 3f9a...
```

## log 日志

日志默认就开着，写在**程序旁边**：

```sh
ggufget get owner/repo -f 1
# 出错时终端会多一行：日志：D:\tools\ggufget\ggufget.log
```

想让程序把一个"看不出来的失败"说清楚（`Access is denied`、镜像返回奇怪状态码、
续传后大小不对），先看这个文件——它记的是终端上不会显示的东西：

- 启动环境：版本、系统、进程号、完整命令行（token 自动打码）、工作目录
- 配置：用的是哪个配置文件、每一项的生效值、模型目录是从哪一级来的
- 每个文件要写到哪个绝对路径、多大、期望的 SHA256
- 每次网络探测：URL、HTTP 状态码、真实总大小、ETag、镜像是否支持分段
- 失败时的完整错误链和 `errno` 数值（`Access is denied` 是 `errno=5`）
- 校验结果、退出码

改位置和详细程度：

```sh
ggufget get owner/repo -f 1 --log ./debug.log   # 指定文件
ggufget get owner/repo -f 1 --log-level debug   # error|warn|info|debug，默认 info
ggufget get owner/repo -f 1 --no-log            # 这次不写
```

也可以用环境变量 `GGUFGET_LOG`、`GGUFGET_LOG_LEVEL`，或配置里的 `log` / `log_level`。
`--verbose` 会把级别临时抬到 `debug`。

写不进程序目录时（只读介质、装在 `Program Files` 又没权限），会自动退到用户缓存目录
（Windows `%LOCALAPPDATA%\ggufget\logs\`，Linux `~/.cache/ggufget/logs/`），再退到临时目录；
三处都写不成时不阻断运行，只在终端告诉你哪几处失败了。

文件超过 4 MiB 会轮转一次，旧的叫 `ggufget.log.1`。

### 配置文件写不进去时

配置默认存在用户配置目录（Windows `%APPDATA%\ggufget\config.json`）。如果这个位置建不了目录
——常见于安全软件拦截未签名程序写用户目录、或域策略锁了漫游配置——程序不会丢设置：
它会退一步写到**程序旁边的 `ggufget.config.json`**，终端提示"已改写到 …"，下次启动自动用这个
便携配置。想显式指定位置就用 `--config <路径>`；指定了还写不进去，就只报错，不再偷偷换地方。

`ggufget doctor` 里"配置文件"一行显示的是**当前生效的那个路径**和它所在目录能不能写。

`ggufget doctor` 会显示当前日志路径和"可写/写不进去"，以及模型目录的写入测试结果。

## lmstudio 和 LM Studio 配合

LM Studio 认这个布局：

```
<模型目录>/<文件名去扩展名>/
└── lmstudio-ai/
    └── <文件名>.gguf
```

所以 ggufget 默认就按这个写。例如下载
`DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf`，文件会落在
`<模型目录>/DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/lmstudio-ai/` 下面。
这两层目录由 ggufget 自动创建，不需要你手动建。

想换摆法用 `--layout`：

- `lmstudio`（默认）：`<文件名去扩展名>/lmstudio-ai/<文件>.gguf`
- `repo`：`<发布者>/<仓库>/<文件>.gguf`
- `flat`：直接放进目标目录，不建子目录（`--no-lmstudio` 等价于此）

布局的默认值也可以写进配置文件的 `layout` 字段，或者用环境变量
`GGUFGET_LAYOUT` 覆盖。

模型目录按下面的顺序找：

1. `--out`
2. 环境变量 `GGUFGET_MODELS_DIR`
3. 配置文件里的 `models_dir`
4. LM Studio 自己的 `~/.lmstudio/settings.json` 里的 `downloadsFolder`
5. `~/.lmstudio/models`

下载完成后，如果本机有 `lms` 命令，ggufget 会问一次索引，告诉你模型已经
出现在 My Models 里。没看到模型时，在 LM Studio 里按一下刷新，或者用
`ggufget doctor` 确认模型目录有没有指对。

## config 配置

先看清楚当前生效的是什么，再决定改什么：

```sh
ggufget config          # 配置文件路径、是否存在、每一项的生效值
ggufget config --json   # 同样的内容，机器可读
ggufget config --init   # 写出默认配置文件（已存在则不动，不会覆盖你的修改）
```

配置文件位置：

- Windows：`%APPDATA%\ggufget\config.json`
- Linux / macOS：`~/.config/ggufget/config.json`

也可以用 `GGUFGET_CONFIG` 指定别的位置，或用 `--config <路径>` 单次覆盖。
文件不存在时按默认值运行，所以不写配置也能直接用。

所有字段都可省略：

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
  "proxy": "",
  "lang": "zh",
  "color": true
}
```

`connections` 和 `jobs` 写 `0` 表示自动（单文件 4 段、同时 1 个文件）。

环境变量会覆盖配置文件：`GGUFGET_PROVIDER`、`GGUFGET_MODELS_DIR`、
`GGUFGET_LAYOUT`、`GGUFGET_HF_ENDPOINT`、`GGUFGET_TIMEOUT`、`GGUFGET_PROXY`、
`GGUFGET_LANG`，以及 HF 的 `HF_TOKEN`。`NO_COLOR=1` 关掉颜色。

## exit-codes 退出码

| 码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 其它错误 |
| 2 | 参数错误（未知选项、缺仓库名） |
| 3 | 网络错误（连不上、超时、429） |
| 4 | 校验失败 |
| 5 | 磁盘空间不足 |
| 6 | 仓库或文件不存在 |
| 7 | 没有写入权限 |
| 130 | 被 Ctrl+C 中断 |

脚本里判断 `if [ $? -eq 5 ]` 就能区分"该清理磁盘"和"该换镜像"。

## troubleshooting 常见问题

**下到一半停了**：直接重跑同一条命令，会从 `.part` 续传。

**速度只有几百 KB/s**：先 `ggufget doctor` 看三个镜像的实测速度；如果
ModelScope 明显更快，用 `-p modelscope`。走代理时记得 `--proxy`。

**报 HTTP 404**：仓库名写错，或者这个仓库只存在于 Hugging Face。用
`ggufget search <关键词>` 找准确 id。

**报私有仓库**：需要 `--token`，或者设置 `HF_TOKEN`。

**连配置都写不进去、连下载目录都建不了，但用资源管理器手动能建**：这种情况
程序本身没问题，是它所在的目录被某个沙箱管住了。典型例子是把 exe 放在
「代理/沙箱工作区」里运行——那类环境会把**放在该目录里的可执行文件**限制成
只能写这个目录，于是 `%APPDATA%`、`~/.lmstudio/models` 全部 `Access is denied`，
而资源管理器（另一个程序）却能建。把 exe 复制到普通目录（例如 `D:\tools\ggufget\`、
桌面）再运行就好了；`ggufget doctor` 的「模型目录/配置文件」两行会直接显示
`可写` 还是 `写不进去`，可以先跑它确认。

**下载完 LM Studio 看不到**：`ggufget doctor` 看模型目录是不是 LM Studio
正在用的那个；`--no-lmstudio` 用过的话，文件会平铺在目录里，LM Studio
不认。

**杀软报毒**：单文件 Go 程序偶尔会被启发式误报；用 `--json` 或源码自己
编译一份即可。

## faq 其它问题

**会不会改 hosts 或系统设置？** 不会。全部行为都在模型目录里。

**支持哪些系统？** Windows（x64/arm64）、Linux（x64/arm64）、macOS
（x64/arm64），都是单文件，无运行时依赖。

**能下非 gguf 文件吗？** `list --any` 能看到全部文件，`-f` 也认完整
文件名。`-a` 只下 gguf。

**怎么把日志留档？** `--log ggufget.log`。

**能放进计划任务里跑吗？** 能：`-y` 跳过确认，退出码区分失败原因，`--json`
给出逐行事件。
