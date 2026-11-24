# 从 5 KB/s 到 8.5 MB/s：我写了一个 GGUF 下载器

> 这篇文章讲一件小事：LM Studio 从国内下载模型太慢。以及一件更小的事：为了把这件事解决干净，最后写了一个跨平台的单文件 CLI。

## 一、一个 7 GB 的模型，要下 2.7 小时

我想在本机试一个 12B 的中文模型。LM Studio 里点下载，进度条走了半小时，涨了 2%。

抓包看了一下：所有下载请求都打到 `https://search.lmstudio.ai:443/v1/hf-proxy/<repo>/resolve/<rev>/<file>`，这是 LM Studio 自家的云代理。实测：

- 拉 256 KB 分片，用了 **48.5 秒**，约 **5 KB/s**；
- 发一个 20 MB 的 Range 请求，读了 15,709,430 字节后直接 `IncompleteRead` 断掉。

按这个速度，7.3 GB 的模型要 **2.7 小时**，而且很可能中途断掉。同一台机器、同一时间段，我直接测别的源：

| 来源 | 实测 |
| --- | --- |
| LM Studio 官方代理 | ~5 KB/s（20 MB 请求直接断流） |
| hf-mirror.com | 0.13 – 3.4 MB/s（随网络波动） |
| ModelScope | 最高 8.5 MB/s |

差了三个数量级。于是问题变成：怎么让 LM Studio 用上别的源？

## 二、走错的两步

### 第一步：改 LM Studio 的配置文件

直觉是"下载地址应该能在设置里换"。翻 `~/.lmstudio/settings.json`，里面确实有个 `useHFProxy: true`，看着很像"使用 HF 代理"。

但它是开关，不是地址。我把 LM Studio 的 `index.js`（35 MB 混淆单行 bundle）扒出来搜：下载 URL 是**硬编码**的 `search.lmstudio.ai:443/v1/hf-proxy`；全文里 `hf-mirror`、`HF_ENDPOINT`、`HF_HOME` 出现 **0 次**。

结论：这个版本的 LM Studio 不提供"换镜像"的配置项。这条路死了。

### 第二步：改 hosts，劫持到本地网关

那就骗它：把 `search.lmstudio.ai` 指向 `127.0.0.1`，本地起个 HTTP 网关，把 `/v1/hf-proxy/<path>` 改成 `https://hf-mirror.com/<path>` 再透传回去。Range、ETag、206 全都原样转发，看起来是个漂亮的方案，也确实跑通了（206 和 HEAD 都正常）。

然后现实来了：

1. **安全软件拦住了 `hosts` 的写入。** 那次之后我花了几个小时排查：ACL 正常、没有 EFS、没有只读属性、`takeown` 说没有特权、`icacls` 说拒绝访问、`CreateFileW` 返回 error 5。最后发现是安全软件的内核驱动在拦——界面上"关掉了"，驱动还在跑。
2. **一次失败的还原把 `hosts` 写成了 0 字节。** 那台机器的域名解析当场全塌。这是我写过的破坏性最强的十几行 PowerShell。
3. **就算成功，LM Studio 的模型搜索也会一起坏。** 搜索和下载共用同一个域名。

我删掉了那个方案的所有代码，并向用户道了歉。教训写进了设计原则：**能用用户态解决的，绝不申请管理员权限；能写自己目录的，绝不写系统目录。**

## 三、正确的解法：绕开它的下载器

关键在于：LM Studio 认模型只认一个东西——**目录布局**。

它自己的 CLI 能列出它索引到的东西：

```json
$ lms ls --json
[{ "path": "Qwen2.5-7B-Instruct-Q4_K_M/lmstudio-ai/Qwen2.5-7B-Instruct-Q4_K_M.gguf", ... }]
```

LM Studio 自己下载时用的就是这套层级：**第一层目录名与文件名（去掉 `.gguf`）相同，第二层固定叫 `lmstudio-ai`**，也就是 `<模型目录>/<文件名>/lmstudio-ai/<文件>.gguf`。按这个摆好，它就会出现在"我的模型"里。（早期手动导入的模型可能是 `<发布者>/<仓库>/<文件>.gguf`，所以工具也保留了 `--layout repo` 这条兼容路径。）

那么问题根本不是"怎么让 LM Studio 换源"，而是"**怎么把文件又快又稳地下到这个目录**"。

于是有了 `ggufget`：

```sh
ggufget get lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f 2
```

不碰 hosts、不碰 LM Studio、不需要管理员权限。卸载就是删一个文件。

## 四、为什么是 Go

最初的版本是三个东西：`hf_download.py`、`modelscope_download.py` 和一个 `.bat` 包装脚本。能用，但发出去就有人问"Python 怎么装"、"为什么 `pip install` 失败"、"Windows 上双击没反应"。

要发给别人，就得是**一个不需要运行时的文件**。候选：

| 方案 | 体积 | 问题 |
| --- | --- | --- |
| Python + PyInstaller | 20–40 MB | 解包到临时目录、启动慢、杀软误报率高 |
| Node + SEA | 60 MB+ | 体积大，构建链复杂 |
| Rust | 2–4 MB | 好，但要引入 HTTP/TLS 等依赖，学习成本高 |
| **Go（stdlib）** | **7.7 MB** | 标准库自带 HTTP/TLS/Range/JSON/SHA256/embed，**零第三方依赖** |

选 Go 的决定性理由是"零依赖"：`go.mod` 里没有一行 `require`，所以构建完全离线、不会因为某个包掉版本而炸。加上：

- `CGO_ENABLED=0` → 静态单文件，六个平台一条 `make dist`；
- 冷启动约 10 ms；
- goroutine 写分段并发下载很顺手；
- 不像 PyInstaller 那样频繁被杀软拦。

## 五、几个值得说的设计决定

### 1. 自动选镜像，但不猜

同时探测三个镜像（ModelScope → hf-mirror → huggingface），按优先级取第一个**真的有这个仓库**的。不是"哪个域名能解析"，而是"哪个镜像的 API 返回了文件列表"。

探测并发，成本约一次往返；结果在同一次运行里缓存。

### 2. 分段下载 + journal

大于 64 MiB 的文件切成 4 段并发 Range 请求。每段进度写进 `.part.seg`：

```json
{"version":1,"total":369358144,"etag":"3ABD...-145",
 "segments":[{"start":0,"length":92339536,"done":44710580}, ...]}
```

`done` 每两秒落盘一次。断网、Ctrl-C、关机之后重跑同一条命令，就从这些位置继续。journal 里存了 `total` 和 `etag`：服务器上的文件变了，就重新切分，绝不把两个版本拼在一起。

### 3. 遇到 429 要会退避，不是硬撞

限流时按 `Retry-After` 退避，并把并发数**减半**。一直用 4 个连接撞一个已经不高兴的服务器，只会得到更多 429。

### 4. 校验分级

`auto`：文件 ≤ 4 GiB，或者这次续传/重试过，才算 SHA256。7 GB 的首次下载跳过全量哈希（省几分钟），因为你已经有 ETag + 大小 + 分段完整性三层保护了。想强制就 `--verify sha256`。

### 5. 完成才改名

先写 `<文件>.part`，大小对、哈希对，才 `rename` 成正式文件名。中途失败不会留下一个"看起来完整"的坏模型——这是最坑人的失败模式。

### 6. 输出分开

结果走 stdout，进度走 stderr。所以：

```sh
ggufget list owner/repo > files.txt      # 文件里只有表格
ggufget get owner/repo -f 2 --json | jq   # NDJSON 事件流
```

### 7. 错误要说人话

三个镜像全挂时，不要只报第一个：

```
✗ 所有镜像都没有取到 no-such/repo：
  modelscope 404 没有这个仓库
  hf-mirror  404 没有这个仓库
  hf         超时
检查网络；或用 --provider 指定单个镜像、--proxy 指定代理后重试。
```

退出码是契约：`0` 成功、`2` 参数错、`3` 网络、`4` 校验失败、`5` 磁盘满、`6` 不存在、`7` 权限、`130` 中断。

### 8. 帮助是命令表，说明书是文件

第一次做的时候我把 500 行说明书 `go:embed` 进了二进制，还自带分页器调 `less` —— 结果用户点开一看字太多，Windows 控制台里还乱码。改了两次才想明白：

- `ggufget help`（交互界面里按 `h`）只回答"这个工具能干什么"：每个命令一行说明、全局选项、退出码。
- 长篇说明书是**外置文件** `MANUAL.md`，随发行包一起给；程序只负责找到它并打印路径（当前目录 → exe 同目录 → `exe/docs`）。
- 分页用内置的（空格翻页、`q` 返回），不调 `less`/`more`：Windows 控制台 + UTF-8 中文 + 外部 pager 这三样凑一起必乱码。

## 六、踩过的坑

- **`416 Range Not Satisfiable` 被当成成功**：差点把截断的文件标记为完成。现在它只被当作"这一段已经下完"。
- **看门狗第一次 tick 就 return**：等于没监控，镜像假死时下载会永远挂着。
- **hf-mirror 的 API 是 308 跳转到 huggingface.co**：所谓"镜像"只镜像了文件，不镜像 API。所以 `search` 在某些网络下会失败，工具会明确告诉你用 `--proxy`。
- **DNS 污染下的 IPv6 陷阱**：AAAA 记录能解析但根本不通，Go 每次拨号都要等一个超时。现在先排序 IPv4 再拨。
- **中文表格对不齐**：CJK 字符占两列，`%-14s` 按字节算，中文界面永远是歪的。按显示宽度补空格。
- **`mkdir …: Access is denied.` 是一句没法查的话**：目录权限、Defender、第三方安全软件、只读介质，全都长这样。用户机器上就是这一行，同一目录用资源管理器能建、LM Studio 自己能写，代码却建不出来——没有更多证据就只能猜。所以 1.2.0 加了默认开启的日志：把命令行、配置生效值、每个落盘绝对路径、探测的状态码/ETag/是否支持分段、错误链和 `errno` 数值全写下来；还在打印计划之前先做一次写入测试，写不进去就直接说"没有写入权限：<目录>"并以 7 退出，而不是让你先看一遍计划再失败。

## 七、怎么用

Windows：

```powershell
irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex
```

Linux / macOS：

```sh
curl -fsSL https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.sh | sh
```

然后：

```sh
ggufget doctor                                    # 先看你这里哪个镜像最快
ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f q4_k_m
```

下载完的文件已经在 LM Studio 的模型目录里，重新打开设置页就能看到。

## 八、如果只记一句话

慢不是最烦的，**不透明**才是：不知道在等什么、不知道会不会断、断了要从头再来、出了错只看到一行 `ConnectionError`。

把这三件事说清楚（在等什么、能不能续、我该怎么办），一个 300 KB/s 的下载器也比一个 8 MB/s 的玄学工具好用。
