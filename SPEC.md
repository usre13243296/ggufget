# ggufget — 命令行契约（SPEC）

本文件是行为的唯一来源。改行为先改这里，再改代码，最后改 `MANUAL.md` / `MANUAL.en.md`（外置说明书）。

版本：1.1.0

---

## 1. 命令面

```
ggufget                         无参数 → 交互式界面
ggufget <command> [options]

list    <repo>      列出文件
get     <repo>      下载
info    <repo>      仓库概览
search  <keyword>   搜索仓库（仅 Hugging Face 系）
doctor              环境自检 + 镜像测速
config              查看/初始化配置文件
version             版本信息
help,h  [command]   帮助总览 / 单个命令的帮助
```

`<repo>` 接受：`owner/name`、`https://huggingface.co/owner/name`、
`https://hf-mirror.com/owner/name`、`https://modelscope.cn/models/owner/name`、
`https://www.modelscope.cn/models/owner/name`、尾部带 `/tree/<rev>` 或 `/resolve/...` 的 URL、
带引号（含中文引号）的字符串。清洗规则见 `provider.NormalizeRepo`。

命令行约定：位置参数可以出现在选项前后（例：`ggufget get -f 2 owner/repo` 与
`ggufget get owner/repo -f 2` 等价）；`--` 之后全部按位置参数处理。

---

## 2. 全局选项

| 长 | 短 | 取值 | 默认 | 说明 |
|---|---|---|---|---|
| `--help` | `-h` | | | 显示帮助 |
| `--version` | `-V` | | | 显示版本 |
| `--quiet` | `-q` | | | 只输出错误 |
| `--verbose` | `-v` | | | 打印 HTTP 细节与重试 |
| `--json` | | | | NDJSON 事件流 |
| `--no-color` | | | | 禁用 ANSI |
| `--no-progress` | | | | 禁用进度动画（仍打印结果） |
| `--lang` | | `zh\|en\|fr\|es\|ru` | 按环境 | 界面语言 |
| `--config` | | 路径 | 平台默认 | 配置文件 |
| `--no-config` | | | | 忽略配置文件 |
| `--log` | | 路径 | 程序旁 `ggufget.log` | 日志文件 |
| `--no-log` | | | | 不写日志 |
| `--log-level` | | `error\|warn\|info\|debug` | `info` | 日志级别（`--verbose` 抬到 debug） |
| `--proxy` | | URL | 环境变量 | HTTP(S) 代理 |
| `--timeout` | | 秒 | 30 | 连接/响应头超时 |

## 3. `list` 选项

| 长 | 短 | 说明 |
|---|---|---|
| `--any` | | 列出所有文件（默认只 `.gguf`） |
| `--filter` | | 关键词过滤（不区分大小写） |
| `--sort` | | `name`\|`size`\|`quant`（默认 `name`） |
| `--limit` | | 最多显示条数 |

序号在过滤+排序之后编号，`get -f <序号>` 以同一规则解释——除非用户显式传了
`--filter/--sort/--limit`，此时序号只在该视图内有效（帮助文本需提示这一点）。

## 4. `get` 选项

| 长 | 短 | 取值 | 默认 | 说明 |
|---|---|---|---|---|
| `--rev` | `-b` | | 自动 | 不传则用列文件时拿到的 commit SHA 钉住 |
| `--file` | `-f` | 规格 | | 可重复 |
| `--all` | `-a` | | | 全部 gguf |
| `--provider` | `-p` | `auto\|modelscope\|hf-mirror\|hf` | `auto` | |
| `--out` | `-o` | 目录 | LM Studio | |
| `--jobs` | `-j` | 整数 | 1 | 并行文件数 |
| `--connections` | `-c` | 整数 | 0=自动 | 单文件分段数；自动 = 4，`< 64 MiB` 或服务端无 Range 时降为 1 |
| `--retries` | | 整数 | 3 | |
| `--verify` | | `auto\|size\|sha256\|off` | `auto` | |
| `--overwrite` | | | | 允许覆盖 |
| `--yes` | `-y` | | | 跳过确认 |
| `--dry-run` | | | | 只打印计划 |
| `--keep-part` | | | | 中断时保留 `.part`（默认也保留，此参数仅为兼容语义） |
| `--token` | | | `HF_TOKEN` | |
| `--layout` | | `lmstudio\|repo\|flat` | `lmstudio` | 落盘目录层级，见 §6 |
| `--no-lmstudio` | | | | 等价于 `--layout flat` |
| `--sha256` | | hex | | 手工期望值 |

### 4.1 `--file` 规格

按顺序尝试，命中即停：

1. **序号表达式**：仅由数字、`,`、`-` 组成。`2`、`1,3`、`1-3`、`3-1`（反向自动纠正）。越界或非法 → 回落到 2。
2. **完整路径或文件名精确匹配**（`Path` 或 `base(Path)`）。
3. **忽略大小写精确匹配**。
4. **唯一包含匹配**（关键词）；命中多个 → 列出候选并失败，不猜。

输入清洗：去掉首尾空格与引号（`'` `"` `'` `"`），剥离前缀
`--file=`、`--file `、`-f `、`file:`、`文件名:`、`文件名 `（不区分大小写），
`\` 归一为 `/`。多个 `--file` 去重保序。

## 5. 镜像

| 名称 | 列表 API | 下载 URL |
|---|---|---|
| `modelscope` | `GET https://modelscope.cn/api/v1/models/{repo}/repo/files?Revision={rev}` → `Data.Files[].{Path,Size,Sha256,Revision}` | `https://modelscope.cn/models/{repo}/resolve/{rev}/{path}` |
| `hf-mirror` | `GET https://hf-mirror.com/api/models/{repo}?blobs=true` → `{sha,siblings[].{rfilename,size,lfs.sha256}}` | `https://hf-mirror.com/{repo}/resolve/{rev}/{path}` |
| `hf` | 同上，域名 `huggingface.co` | `https://huggingface.co/{repo}/resolve/{rev}/{path}` |

- `auto`：按 `modelscope → hf-mirror → hf` 顺序探测（各 3 s 超时，并发），取第一个存在的。
- 列表请求不带 revision → 拿默认分支；响应里的 `sha`（ModelScope 是 `Files[].Revision`）作为下载用的 `rev`。
- 两个镜像都支持 `resolve/<commit-sha>/<path>`（已实测 206）。
- `search` 只走 HF 接口：`GET {endpoint}/api/models?search=<q>&limit=<n>`。

## 6. 目录与落盘

模型目录解析顺序：

1. `--out`
2. `GGUFGET_MODELS_DIR`
3. 配置 `models_dir`
4. LM Studio `settings.json` 的 `downloadsFolder`（三平台均为 `~/.lmstudio/settings.json`）
5. `~/.lmstudio/models`
6. 都没有 → 报错并提示 `--out`

落盘布局由 `--layout`（或配置 `layout`、`GGUFGET_LAYOUT`）决定，默认 `lmstudio`：

```
lmstudio（默认）:
<models_dir>/<file-stem>/lmstudio-ai/<file>.gguf
repo:
<models_dir>/<publisher>/<repo>/<file>.gguf
flat:
<models_dir>/<file>.gguf
```

`<file-stem>` = 文件名去掉 `.gguf`（大小写不敏感），即第一层目录名与文件名（去扩展名）相同；
第二层固定为 `lmstudio-ai`。这是 LM Studio 识别模型要求的层级，两层目录都必须存在，
由 `os.MkdirAll` 自动创建。

`publisher` 与 `repo` 来自仓库 id 的两段，仅 `repo` 布局使用。文件内的子目录在 `lmstudio`
与 `flat` 布局下会拍平（取 `path.Base`），在 `repo` 布局下保留（`Path` 里的 `/` → 平台分隔符）。
多个文件解析出同一目标路径时报冲突错误，不静默覆盖。

下载中间文件：目标名 + `.part`；分段状态：目标名 + `.part.seg`（JSON）。完成后
`Rename(.part → 目标)`，并删除 `.part.seg`。**任何时刻都不存在半截的目标文件。**

## 7. 校验策略

- 大小永远比对。
- `--verify auto`（默认）：**满足任一条件才算 SHA256** —— 目标 ≤ 4 GiB、本次发生过续传或重试、或镜像没给 SHA256 之外的情形下无值可算。
- `--verify size`：只比大小。
- `--verify sha256`：强制算，缺期望值时只报告实际值并失败（提示用 `--sha256`）。
- `--verify off`：都不算（仍比大小，大小是下载器自身的正确性判据）。
- 失败 → 删除 `.part` 重下一次；再失败 → 退出码 4。

## 8. 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 通用错误 |
| 2 | 参数/用法错误 |
| 3 | 网络/HTTP 错误（重试耗尽） |
| 4 | 校验失败 |
| 5 | 磁盘不足 |
| 6 | 仓库或文件不存在 |
| 7 | 权限/路径不可写 |
| 130 | 用户中断（Ctrl-C / SIGINT） |

## 9. 输出

- **stdout**：结果（列表、计划、最终路径、`--json` 事件）。
- **stderr**：进度与错误。
- TTY：单行动画，`\r` 覆盖；宽度 < 60 用紧凑布局；不支持 ANSI 时用 `#`/`.` 进度条。
- 非 TTY：每 10 s 一行，结束一行总结；无 `\r`、无 ANSI。
- `--json`：每行一个对象，`event` ∈ `start|progress|done|error|saved|plan|list`。
  字段：`file,index,count,total,done,speed,conns,elapsed,path,error`。

## 10. 配置

路径：Windows `%APPDATA%\ggufget\config.json`；其他 `${XDG_CONFIG_HOME:-~/.config}/ggufget/config.json`。

```json
{
  "provider": "auto",
  "hf_endpoint": "https://hf-mirror.com",
  "ms_endpoint": "https://modelscope.cn",
  "models_dir": "",
  "layout": "lmstudio",
  "connections": 0,
  "jobs": 1,
  "retries": 3,
  "timeout": 30,
  "verify": "auto",
  "proxy": "",
  "lang": "zh",
  "color": true,
  "token": "",
  "history": ["Qwen/Qwen2.5-7B-Instruct-GGUF"],
  "speeds": {"modelscope": 8.5, "hf-mirror": 0.7},
  "speeds_at": "2026-11-22T22:00:00+08:00"
}
```

`history`（TAB 补全用，最多 200 条）与 `speeds`/`speeds_at`（启动时显示的镜像速度，单位 MiB/s）
由交互式界面写入，手改无害。`lang` 取 `zh`/`en`/`fr`/`es`/`ru`，空 = 跟随系统区域设置。

优先级：**命令行 > 环境变量 > 配置文件 > 内置默认**。
环境变量：`GGUFGET_*`（同上键名大写）、`HF_TOKEN`、`HTTP_PROXY`、`HTTPS_PROXY`、`NO_PROXY`、`NO_COLOR`、`COLUMNS`。
配置文件损坏 → stderr 告警 + 用默认值，不退出。

## 11. 下载引擎

1. 探测：`GET Range: bytes=0-0`，读 `Content-Range: bytes 0-0/<total>`、`ETag`、`Accept-Ranges`。
2. 分段条件：`connections > 1` && `total ≥ 64 MiB` && 服务端支持 Range。否则单连接。
3. 分段：均分 N 段；目标文件 `Truncate(total)` 预分配；每段 `WriteAt`。
   状态写 `.part.seg`（`{version,total,etag,segments:[{start,length,done}]}`），每 2 s 或每段完成时落盘。
   续传：`.part.seg` 与探测结果的 `total`/`etag` 一致才续；否则从头。
4. 单连接：`.part` 大小即进度，直接 `Range: bytes=<size>-`。
5. 服务端忽略 Range 返回 200 → 截断重下，打印说明。
6. `416` → 视为已完整，走校验。
7. 重试：`--retries` 次，退避 `500ms * 2^n` + 抖动（上限 30 s），尊重 `Retry-After`。
8. 429/503：读 `Retry-After`（缺省 5 s），并把本文件连接数减半重试（最低 1）。
9. ETag 中途变化 → 放弃 `.part` 从头。
10. 连接空闲 15 s 无字节 → 断线重连该段。
11. 磁盘预检：需要 `total * 1.05 + 16 MiB`，不足则退出码 5（不开始）。

## 12. 交互式界面（无参数）

启动时打印头部：版本、模型目录、镜像（含上次测到的速度）、当前语言。

```
[1] 下载模型     → 输入仓库 → 列出文件 → 输入序号/关键词 → 计划 → 确认
[2] 搜索仓库     → 关键词 → 编号列表 → 选一个 → 同上
[3] 测速         → 串行测 modelscope / hf-mirror / hf，结果写回配置
[4] 设置         → [1]语言 [2]布局 [3]镜像 [4]模型目录
[h] 帮助         → 内置分页器显示总览
[q] 退出
```

交互约定（所有菜单一致）：

- 输入键（`1`、`q`）或选项文字（`下载模型` / `download`）都能选中，不区分大小写。
- `h` 在菜单内打开帮助，`q` 退出当前层；空行等价于返回。
- 输入行支持行编辑：←/→/Home/End、Backspace/Delete、Ctrl+A/E/K/U/W；↑/↓ 翻历史。
- `TAB` 补全：仓库 id 的历史、命令名、`--layout`/`--provider`/语言的值、当前层的选项键。
- Ctrl+C 立即中断并回到上一层（下载中为取消，退出码 130）。
- 语言在设置里切换后立即生效并写回配置。
- 非 TTY（管道/重定向）时退化为按行读取，不进入 raw 模式。
- 双击启动（控制台里只有本进程）时退出前停住等回车。

## 13. 帮助分层

- `ggufget`（无参数）→ 交互式界面，`h` 看总览。
- `ggufget help` / `ggufget h` / `ggufget --help`：命令总表（每条一行说明）、全局选项、
  三条上手提示、退出码、外部说明书的位置；在交互式界面里用内置分页器。
- `ggufget help <命令>`：该命令的用法、全部选项、3 个示例、相关退出码。
- 说明书是**外置文档**：`MANUAL.md` / `MANUAL.en.md`，随发行包提供；
  程序只打印它的位置（exe 同目录 → exe/docs → 当前目录），不把长文编译进二进制。
- 选项拼错：Levenshtein ≤ 2 时提示 `你是不是想输入 --xxx ?`

## 14. 日志

- 默认开启；路径优先级：`--log <路径>` > `GGUFGET_LOG` > 配置 `log` >
  `<exe 目录>/ggufget.log` > `<用户缓存>/ggufget/logs/ggufget.log` > `<临时目录>/ggufget.log`。
  全部失败不阻断运行，只在 stderr 逐条打印失败路径。
- 级别：`error` / `warn` / `info`（默认）/ `debug`；`--verbose` 把本次抬到 `debug`。
- 行格式：`<RFC3339 毫秒带时区> <LEVEL> <message>`，消息内换行替换为 ` | `。
- 单文件超过 4 MiB 轮转一次为 `<名称>.1`（旧的被覆盖）。
- 必须记录：启动（版本/系统/架构/pid/可执行文件/工作目录/脱敏后的 argv）、
  配置（路径、各项生效值、来源告警）、模型目录的来源判断、`layout`、每个落盘绝对路径、
  每次网络探测（URL/状态码/总大小/ETag/是否支持分段）、`mkdir` 等文件系统失败的
  完整错误链与 errno 数值、续传与 journal 命中的原因、重试与限流、校验结果、退出码。
- 命令失败时终端额外打印一行 `日志：<实际路径>`；`doctor` 显示日志路径与
  "可写/写不进去"，并对模型目录做写入测试。
- 开工前先做写入测试：模型目录不可写时**不打印计划**，直接以退出码 7 结束，
  并给出 `没有写入权限：<目录>（换 --out 或检查目录权限）`。`--dry-run` 不做写入测试。
- 凭据脱敏：`hf_*` token、`token=`/`api_key=`/`Authorization:`/`Bearer` 的值一律替换为 `***`。
- 配置写入失败必须记日志（含错误链与 errno）。默认配置位置不可写时，退到
  `<exe 目录>/ggufget.config.json`（便携配置），终端提示"已改写到 <路径>"，并在
  下次启动时优先使用该便携文件；`--config <路径>` 或 `GGUFGET_CONFIG` 显式指定的位置
  不可写时只报错，不再换位置。`doctor` 的"配置文件"行显示实际生效路径与其可写性。
