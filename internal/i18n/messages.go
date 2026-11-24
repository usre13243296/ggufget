package i18n

// Register fills the message tables. Called from init.
func Register() {
	// ----- identity -------------------------------------------------------
	def("app.tagline",
		"把 GGUF 模型从国内镜像快速拉进 LM Studio",
		"Pull GGUF models from fast mirrors into LM Studio")
	def("app.summary",
		"支持 ModelScope / hf-mirror / huggingface.co，自动选最快的那个。",
		"Works with ModelScope, hf-mirror and huggingface.co; picks the fastest one.")

	// ----- usage ----------------------------------------------------------
	def("usage.line", "用法：", "Usage:")
	def("usage.body", `  ggufget                         交互式界面（双击也能用）
  ggufget <命令> [选项]

命令：
  list    <仓库>     列出文件（序号 / 大小 / 量化）
  get     <仓库>     下载（例：ggufget get owner/repo -f 2）
  info    <仓库>     仓库概览：默认分支、许可、总大小、哪些镜像有
  search  <关键词>   搜索仓库（走 Hugging Face 搜索接口）
  doctor             环境自检 + 镜像测速
  config             显示当前配置生效值及其文件位置
  version            版本信息
  help,h  [命令]     帮助总览 / 单个命令的完整帮助`, `  ggufget                         interactive shell (double-click friendly)
  ggufget <command> [options]

Commands:
  list    <repo>     list files (index / size / quant)
  get     <repo>     download (e.g. ggufget get owner/repo -f 2)
  info    <repo>     repo overview: branch, license, size, mirrors that have it
  search  <keyword>  search repos (Hugging Face search API)
  doctor             environment check + mirror speed test
  config             show the effective config and its file
  version            version info
  help,h  [command]  help overview / full help for one command`)
	def("usage.examples", "示例：", "Examples:")
	def("usage.examples.body", `  ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
  ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f q4_k_m
  ggufget get  owner/repo -f 2 --dry-run
  ggufget doctor
  ggufget help get`, `  ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
  ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f q4_k_m
  ggufget get  owner/repo -f 2 --dry-run
  ggufget doctor
  ggufget help get`)

	// ----- command descriptions ------------------------------------------
	def("cmd.list", "列出仓库里的文件", "list files in a repo")
	def("cmd.get", "下载到 LM Studio 模型目录", "download into the LM Studio models dir")
	def("cmd.info", "仓库概览", "repo overview")
	def("cmd.search", "搜索仓库", "search repos")
	def("cmd.doctor", "环境自检", "environment check")
	def("cmd.config", "显示配置与配置文件位置", "show the effective config and its file")
	def("cmd.version", "版本信息", "version info")
	def("cmd.help", "帮助总览与单命令帮助", "help overview and per-command help")

	// ----- generic flags --------------------------------------------------
	def("flag.help", "显示帮助", "show help")
	def("flag.version", "显示版本", "show version")
	def("flag.quiet", "只输出错误", "errors only")
	def("flag.verbose", "打印 HTTP 细节与重试", "print HTTP details and retries")
	def("flag.json", "NDJSON 事件流（每行一个 JSON）", "NDJSON event stream (one JSON per line)")
	def("flag.nocolor", "禁用颜色", "disable color")
	def("flag.noprogress", "禁用进度动画", "disable the progress display")
	def("flag.lang", "界面语言 zh|en|fr|es|ru", "interface language zh|en|fr|es|ru")
	def("flag.config", "配置文件路径", "config file path")
	def("flag.noconfig", "忽略配置文件", "ignore the config file")
	def("flag.log", "日志文件（默认写在程序旁边）", "log file (default: next to the binary)")
	def("flag.nolog", "不写日志", "do not write a log")
	def("flag.loglevel", "日志级别 error|warn|info|debug", "log level error|warn|info|debug")
	def("flag.proxy", "HTTP 代理（覆盖环境变量）", "HTTP proxy (overrides env)")
	def("flag.timeout", "连接/响应头超时秒数", "connect/header timeout in seconds")

	def("flag.rev", "版本/分支；默认自动钉住当前 commit", "revision; defaults to the current commit")
	def("flag.file", "选哪个文件：序号/关键词/文件名，可重复", "which file: index / keyword / name; repeatable")
	def("flag.all", "下载全部 gguf", "download every gguf")
	def("flag.provider", "auto|modelscope|hf-mirror|hf", "auto|modelscope|hf-mirror|hf")
	def("flag.out", "输出目录（默认 LM Studio 模型目录）", "output dir (default: LM Studio models dir)")
	def("flag.jobs", "同时下载几个文件", "how many files at once")
	def("flag.conns", "单文件分段连接数（0=自动）", "segmented connections per file (0=auto)")
	def("flag.retries", "失败重试次数", "retry attempts")
	def("flag.verify", "auto|size|sha256|off", "auto|size|sha256|off")
	def("flag.overwrite", "允许覆盖已存在的文件", "allow overwriting existing files")
	def("flag.yes", "不再确认", "skip confirmation")
	def("flag.dryrun", "只显示计划，不下载", "print the plan, download nothing")
	def("flag.keeppart", "中断时保留 .part 文件", "keep .part files on interrupt")
	def("flag.token", "访问令牌（也可用 HF_TOKEN）", "access token (or HF_TOKEN)")
	def("flag.nolmstudio", "直接放进目标目录，不建任何子目录", "drop the file in the target dir, no subdirs")
	def("flag.layout", "目录布局：lmstudio（默认）|repo|flat", "layout: lmstudio (default) | repo | flat")
	def("flag.sha256", "手工指定期望的 SHA256", "expected SHA256")
	def("flag.any", "列出所有文件（默认只看 .gguf）", "list every file (default: only .gguf)")
	def("flag.filter", "关键词过滤", "keyword filter")
	def("flag.sort", "name|size|quant", "name|size|quant")
	def("flag.limit", "最多显示几条", "maximum rows")
	def("flag.update", "检查并安装新版本", "check for and install a newer release")

	// ----- errors ---------------------------------------------------------
	def("err.usage", "参数有误：%s", "bad arguments: %s")
	def("err.unknownCmd", "未知命令：%s", "unknown command: %s")
	def("err.suggestCmd", "你是不是想输入 %s ?", "did you mean %s ?")
	def("err.unknownFlag", "未知选项：%s", "unknown option: %s")
	def("err.suggestFlag", "你是不是想输入 %s ?", "did you mean %s ?")
	def("err.tryHelp", "运行 ggufget help 查看用法。", "run ggufget help for usage.")
	def("err.repoRequired", "缺少仓库名。", "missing repo name.")
	def("err.repoFormat",
		"仓库名要写成 owner/name，例如 lmstudio-community/Qwen2.5-7B-Instruct-GGUF",
		"repo must look like owner/name, e.g. lmstudio-community/Qwen2.5-7B-Instruct-GGUF")
	def("err.needFile", "没说要下哪个文件：用 -f <序号|关键词> 或 -a 下全部。",
		"nothing selected: use -f <index|keyword> or -a for everything.")
	def("err.fileAndAll", "-f 和 -a 不能同时用。", "-f and -a cannot be combined.")
	def("err.network", "网络错误：%v", "network error: %v")
	def("err.http", "HTTP %d：%s", "HTTP %d: %s")
	def("err.notFound", "仓库或文件不存在（HTTP 404）：%s", "repo or file not found (HTTP 404): %s")
	def("err.notFoundText", "没有这个仓库（HTTP 404）。", "no such repo (HTTP 404).")
	def("err.notFoundHint",
		"检查是否写成 owner/name；只记得关键词的话试 ggufget search <关键词>。",
		"check the owner/name spelling; if you only remember a keyword try ggufget search <keyword>.")
	def("err.mirrorsFailed", "所有镜像都没有取到 %s：", "no mirror could serve %s:")
	def("err.mirrorsHint", "检查网络；或用 --provider 指定单个镜像、--proxy 指定代理后重试。",
		"check your network, or retry with --provider to pin one mirror or --proxy to set a proxy.")
	def("err.http404", "404 没有这个仓库", "404 not found")
	def("err.authShort", "这个镜像需要授权", "auth required on this mirror")
	def("err.timeout", "超时", "timeout")
	def("err.private", "这是私有/需要授权的仓库，用 --token 或设置 HF_TOKEN。",
		"private or gated repo; pass --token or set HF_TOKEN.")
	def("err.diskFull", "磁盘空间不足：需要 %s，还剩 %s。", "not enough disk space: need %s, %s free.")
	def("err.permission", "没有写入权限：%s（换 --out 或检查目录权限）",
		"permission denied writing %s (use --out or fix the directory permissions)")
	def("err.verifySize", "大小不符：期望 %d 字节，实际 %d 字节。", "size mismatch: expected %d bytes, got %d.")
	def("err.verifySHA", "SHA256 不符：期望 %s，实际 %s。", "SHA256 mismatch: expected %s, got %s.")
	def("err.interrupt", "已中断。已下载的部分保留在 %s，重新运行可续传。",
		"interrupted. Partial data kept in %s; rerun to resume.")
	def("err.noModelsDir", "找不到 LM Studio 模型目录，用 --out 指定一个。",
		"cannot find the LM Studio models dir; pass --out.")
	def("err.badLayout", "不认识的目录布局：%s（可选 lmstudio、repo、flat）。",
		"unknown layout: %s (expected lmstudio, repo or flat).")
	def("err.destCollision", "%s 和 %s 会写到同一个位置，请分开下载。",
		"%s and %s would land in the same path; download them separately.")
	def("err.selectNoMatch", "没有匹配 %q 的文件。", "no file matches %q.")
	def("err.selectAmbiguous", "%q 匹配到 %d 个文件，请写具体些：", "%q matches %d files, please be more specific:")
	def("err.selectHint",
		"提示：可以填列表序号（2、1,3、1-3），也可以填关键词（q4_k_m）；先运行 ggufget list <仓库> 看全部文件。",
		"hint: use a list index (2, 1,3, 1-3) or a keyword (q4_k_m); run ggufget list <repo> to see every file.")
	def("err.allFailed", "全部失败。", "all downloads failed.")
	def("err.partial", "%d 个成功，%d 个失败。", "%d succeeded, %d failed.")
	def("err.logPath", "日志：%s", "log: %s")
	def("err.logDisabled", "无法写日志 %s（%s）", "cannot write the log %s (%s)")
	def("err.badLoglevel", "不认识的日志级别：%s（可选 error、warn、info、debug）。",
		"unknown log level: %s (choose error, warn, info or debug).")

	// ----- list -----------------------------------------------------------
	def("list.title", "[%s] 共 %d 个文件，合计 %s", "[%s] %d files, %s total")
	def("list.titleGGUF", "[%s] 共 %d 个 gguf，合计 %s（其余文件用 --any 查看）",
		"[%s] %d gguf files, %s total (use --any for the rest)")
	def("list.index", "序号", "idx")
	def("list.file", "文件", "file")
	def("list.size", "大小", "size")
	def("list.quant", "量化", "quant")
	def("list.empty", "这个仓库里没有匹配的文件。", "no matching files in this repo.")
	def("list.pickHint", "下载：-f <序号>  例如  ggufget get %s -f 2",
		"download: -f <index>   e.g.  ggufget get %s -f 2")

	// ----- plan -----------------------------------------------------------
	def("plan.title", "下载计划", "download plan")
	def("plan.repo", "仓库", "repo")
	def("plan.provider", "镜像", "mirror")
	def("plan.rev", "版本", "revision")
	def("plan.dest", "落盘", "destination")
	def("plan.layout", "布局", "layout")
	def("plan.files", "文件", "files")
	def("plan.total", "合计", "total")
	def("plan.eta", "按 %.1f MiB/s 估计约 %s", "about %s at %.1f MiB/s")
	def("plan.noSpeed", "耗时取决于镜像速度", "time depends on mirror speed")
	def("plan.confirm", "开始下载？[Y/n] ", "start? [Y/n] ")
	def("plan.cancelled", "已取消。", "cancelled.")

	// ----- download -------------------------------------------------------
	def("dl.fileHeader", "[%d/%d] %s  (%s)", "[%d/%d] %s  (%s)")
	def("dl.resume", "从 %s 续传", "resuming at %s")
	def("dl.skip", "已存在且完整，跳过：%s", "already complete, skipping: %s")
	def("dl.verify", "校验中…", "verifying...")
	def("dl.verified", "校验通过", "verified")
	def("dl.done", "完成，用时 %s", "done in %s")
	def("dl.retry", "第 %d 次重试（%s 后）：%v", "retry %d (after %s): %v")
	def("dl.conns", "%d 连接", "%d connections")
	def("dl.ratelimit", "被限速（HTTP 429），降到 %d 连接重试", "rate limited (HTTP 429), retrying with %d connections")
	def("dl.dryrun", "（dry-run，不下载）", "(dry-run, nothing downloaded)")
	def("dl.summary", "完成：%d 个文件，%s，用时 %s", "done: %d files, %s, in %s")

	// ----- lmstudio -------------------------------------------------------
	def("lm.visible", "LM Studio → My Models 已经能看到（lms 已索引）。",
		"LM Studio sees it already (indexed by lms).")
	def("lm.hint", "打开 LM Studio → My Models 即可加载。", "open LM Studio -> My Models to load it.")
	def("lm.dir", "模型目录", "models dir")
	def("lm.notRunning", "（未检测到 lms 命令，跳过索引校验）", "(lms not found, skipping index check)")

	// ----- doctor ---------------------------------------------------------
	def("doc.title", "环境自检", "environment check")
	def("doc.os", "系统", "system")
	def("doc.disk", "磁盘剩余", "free disk")
	def("doc.modelsdir", "模型目录", "models dir")
	def("doc.lms", "LM Studio", "LM Studio")
	def("doc.proxy", "代理", "proxy")
	def("doc.proxyNone", "未设置", "not set")
	def("doc.test", "测速（下载 8 MiB）", "speed test (8 MiB)")
	def("doc.recommend", "建议用 %s（%.1f MiB/s）", "recommended: %s (%.1f MiB/s)")
	def("doc.noMirror", "所有镜像都连不上。检查网络或代理。", "no mirror reachable; check network or proxy.")
	def("doc.ok", "正常", "ok")
	def("doc.bad", "不可用", "unavailable")
	def("doc.speed", "%s  %.1f MiB/s", "%s  %.1f MiB/s")
	def("doc.speedFail", "%s  连不上（%v）", "%s  unreachable (%v)")
	def("doc.configPath", "配置文件", "config file")
	def("doc.logPath", "日志文件", "log file")
	def("doc.logOK", "可写", "writable")
	def("doc.logBad", "写不进去", "not writable")
	def("doc.logOff", "已关闭", "off")
	def("doc.go", "构建", "build")

	// ----- info -----------------------------------------------------------
	def("info.title", "仓库概览", "repo overview")
	def("info.gated", "需要授权（gated）", "gated (needs access approval)")
	def("info.private", "私有仓库", "private repo")
	def("info.license", "许可", "license")
	def("info.gguf", "%d 个 gguf，合计 %s", "%d gguf files, %s total")
	def("info.quant", "量化", "quantizations")
	def("info.largest", "最大", "largest")
	def("info.smallest", "最小", "smallest")
	def("info.tokenHint", "设置 HF_TOKEN 或用 --token 后重试。", "set HF_TOKEN or pass --token, then retry.")

	// ----- config ---------------------------------------------------------
	def("cfg.loaded", "已读取配置：%s", "config loaded: %s")
	def("cfg.bad", "配置文件有问题，已忽略：%v", "config file ignored: %v")
	def("cfg.hint", "用 ggufget doctor 看看环境；配置示例见 ggufget help config。",
		"try ggufget doctor; see ggufget help config for an example")
	def("cfg.title", "配置：", "config:")
	def("cfg.exists", "文件已存在", "file exists")
	def("cfg.missing", "文件还不存在，当前用的是默认值", "file does not exist yet, defaults in use")
	def("cfg.initDone", "已写入默认配置文件：%s", "wrote the default config file: %s")
	def("cfg.initExists", "配置文件已存在，没有覆盖：%s", "config file already exists, left untouched: %s")
	def("cfg.provider", "镜像", "provider")
	def("cfg.hfEndpoint", "hf 镜像地址", "hf endpoint")
	def("cfg.msEndpoint", "modelscope", "modelscope")
	def("cfg.modelsDir", "模型目录", "models dir")
	def("cfg.layout", "目录布局", "layout")
	def("cfg.connections", "分段连接数", "connections")
	def("cfg.jobs", "并行文件数", "jobs")
	def("cfg.retries", "重试次数", "retries")
	def("cfg.timeout", "超时(秒)", "timeout (s)")
	def("cfg.verify", "校验策略", "verify")
	def("cfg.lang", "界面语言", "language")
	def("cfg.log", "日志文件", "log file")
	def("cfg.logLevel", "日志级别", "log level")
	def("cfg.auto", "自动", "auto")
	def("flag.init", "写出默认配置文件（已存在则不覆盖）", "write a default config file (never overwrites)")

	// ----- select ---------------------------------------------------------
	def("sel.index", "序号 %s -> %s", "index %s -> %s")
	def("sel.fuzzy", "%q -> %s", "%q -> %s")

	// ----- search ---------------------------------------------------------
	def("search.title", "搜索 %q 找到 %d 个仓库：", "%d repos match %q:")
	def("search.none", "没搜到 %q 的仓库。", "no repos match %q.")
	def("search.note", "搜索只覆盖 Hugging Face（hf-mirror 与 hf.co 同一套接口）。ModelScope 请直接粘仓库 id。",
		"search covers Hugging Face only (hf-mirror and hf.co share the API). For ModelScope, paste the repo id.")
	def("search.failHint", "搜索走的是 Hugging Face 的 API（hf-mirror 也会把 API 请求跳转到 hf.co）。连不上时试试 --proxy，或直接用 ggufget list <仓库 id>。",
		"search uses the Hugging Face API (hf-mirror redirects it to hf.co). If it is unreachable, try --proxy, or pass the repo id straight to ggufget list.")

	// ----- help -----------------------------------------------------------
	def("help.usageOf", "用法：ggufget %s", "Usage: ggufget %s")
	def("help.options", "选项", "Options")
	def("help.examples", "示例", "Examples")
	def("help.value", "值", "value")
	def("help.exitcodes",
		"退出码：0 成功 · 2 参数错误 · 3 网络 · 4 校验失败 · 5 磁盘不足 · 6 不存在 · 7 权限 · 130 中断",
		"Exit codes: 0 ok · 2 usage · 3 network · 4 verify · 5 disk · 6 missing · 7 permission · 130 interrupt")
	def("help.ex.list", `  ggufget list owner/repo
  ggufget list owner/repo --sort size
  ggufget list owner/repo --filter q4`, `  ggufget list owner/repo
  ggufget list owner/repo --sort size
  ggufget list owner/repo --filter q4`)
	def("help.ex.get", `  ggufget get owner/repo -f 2
  ggufget get owner/repo -f q4_k_m -y
  ggufget get owner/repo --all -j 2
  ggufget get owner/repo -f 1 --dry-run`, `  ggufget get owner/repo -f 2
  ggufget get owner/repo -f q4_k_m -y
  ggufget get owner/repo --all -j 2
  ggufget get owner/repo -f 1 --dry-run`)
	def("help.ex.info", `  ggufget info owner/repo
  ggufget info https://huggingface.co/owner/repo`, `  ggufget info owner/repo
  ggufget info https://huggingface.co/owner/repo`)
	def("help.ex.search", `  ggufget search qwen2.5-7b
  ggufget search "deepseek coder"`, `  ggufget search qwen2.5-7b
  ggufget search "deepseek coder"`)
	def("help.ex.doctor", `  ggufget doctor
  ggufget doctor --verbose`, `  ggufget doctor
  ggufget doctor --verbose`)
	def("help.ex.config", `  ggufget config
  ggufget config --init
  ggufget config --json`, `  ggufget config
  ggufget config --init
  ggufget config --json`)
}
