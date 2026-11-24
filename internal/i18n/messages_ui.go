package i18n

// Interactive shell and help text. Kept apart from messages.go so translators
// can work on one compact file; the long-form manual is NOT in here, it ships as
// README.md / MANUAL.md next to the source.
func init() { RegisterUI() }

// RegisterUI fills the strings used by the interactive shell and `help`.
func RegisterUI() {
	def("ui.prompt", "选择 › ", "choice › ")
	def("ui.mainHint", "输入编号回车；h 帮助；q 退出", "type a number, h for help, q to quit")
	def("ui.bye", "再见。", "Bye.")
	def("ui.back", "返回", "Back")

	def("ui.cmdDownload", "下载模型", "Download a model")
	def("ui.cmdDownloadHint", "输入仓库，先列文件再选", "enter a repo, list files, pick one")
	def("ui.cmdSearch", "搜索模型", "Search models")
	def("ui.cmdSearchHint", "按关键词找 Hugging Face 仓库", "find repos on Hugging Face")
	def("ui.cmdSpeed", "镜像测速", "Measure mirrors")
	def("ui.cmdSpeedHint", "重测 ModelScope / hf-mirror / HF", "re-test ModelScope / hf-mirror / HF")
	def("ui.cmdSettings", "设置", "Settings")
	def("ui.cmdSettingsHint", "语言 / 布局 / 镜像 / 目录", "language / layout / mirror / folder")
	def("ui.cmdHelp", "帮助", "Help")
	def("ui.cmdHelpHint", "列出全部命令与用法", "list every command and its options")
	def("ui.cmdQuit", "退出", "Quit")

	def("ui.modelsDir", "模型目录", "models")
	def("ui.modelsDirNone", "未找到（可用 --out 指定）", "not found (pass --out)")
	def("ui.mirrors", "镜像", "mirrors")
	def("ui.lang", "语言", "language")
	def("ui.speedNone", "未测速（在菜单里按 3 测一次）", "not measured yet (press 3 in the menu)")
	def("ui.speedFail", "连不上", "unreachable")
	def("ui.speedRunning", "正在测速，每个镜像取样 4 MiB …", "measuring each mirror with a 4 MiB sample …")
	def("ui.speedSaved", "速度已记住，下次启动直接显示。", "speeds saved and shown at next start.")
	def("ui.saveFailed", "保存失败：%s", "could not save: %s")
	def("ui.saveFallback", "默认位置写不进去，已改写到 %s", "default location not writable, saved to %s")
	def("ui.saved", "已保存。", "Saved.")
	def("ui.badChoice", "没有这个选项：%s", "no such option: %s")

	def("ui.repoPrompt", "仓库（作者/名字，或粘贴链接）› ", "repo (owner/name or a URL) › ")
	def("ui.selectPrompt", "编号（如 1,3 / a 全部 / q 返回）› ", "number (1,3 / a all / q back) › ")
	def("ui.selectRetry", "再输一次，或按 q 返回。", "try again, or q to go back.")
	def("ui.searchPrompt", "关键词 › ", "keyword › ")
	def("ui.searchPick", "选一个编号进入仓库（q 返回）› ", "pick a number to open it (q goes back) › ")

	def("ui.setTitle", "设置", "Settings")
	def("ui.setLang", "语言", "Language")
	def("ui.setLayout", "目录布局", "Layout")
	def("ui.setProvider", "默认镜像", "Default mirror")
	def("ui.setModelsDir", "模型目录", "Models folder")
	def("ui.setLangPrompt", "语言 › ", "language › ")
	def("ui.setLayoutPrompt", "布局 › ", "layout › ")
	def("ui.setProviderPrompt", "镜像 › ", "mirror › ")
	def("ui.setDirPrompt", "新目录（留空不改）› ", "new folder (empty keeps it) › ")
	def("ui.layoutLMSHint", "LM Studio 推荐的层级", "what LM Studio expects")
	def("ui.layoutRepoHint", "旧式 发布者/仓库 树", "old publisher/repo tree")
	def("ui.layoutFlatHint", "直接放一个文件", "one flat file")
	def("ui.providerAutoHint", "自动挑最快的", "pick the fastest")
	def("ui.providerMSHint", "国内通常最快", "usually fastest in China")
	def("ui.providerHFMHint", "文件最全，速度波动", "most complete, speed varies")
	def("ui.providerHFHint", "官方源", "official source")
	def("ui.more", "空格 继续 / q 返回", "space: more / q: back")

	def("help.commands", "命令", "Commands")
	def("help.globalOptions", "全局选项（所有命令都接受）", "Global options (accepted by every command)")
	def("help.tips", "提示", "Tips")
	def("help.tipHelp", "交互模式里按 h 看这份清单，q 返回上一级，主菜单按 q 退出。",
		"in the interactive shell h shows this list, q goes back one level, and q at the main menu exits.")
	def("help.tipTab", "输入框支持 TAB 补全（历史仓库）、上下键翻历史、左右键与 Home/End 移动光标、Ctrl+W 删词。",
		"prompts support TAB completion (recent repos), up/down history, left/right and Home/End, and Ctrl+W to delete a word.")
	def("help.tipDocs", "说明书是外置文档，不打包进程序：README.md 快速上手，MANUAL.md 详细手册。",
		"the manual is an external document, not baked into the binary: README.md to start, MANUAL.md for details.")
	def("help.docs", "文档", "Documentation")
	def("help.docsWhere", "仓库根目录的 README.md 与 MANUAL.md（英文版 MANUAL.en.md）。",
		"README.md and MANUAL.md at the repository root (English: MANUAL.en.md).")
	def("help.getIndexRule",
		"序号按与 list 相同的视图解释：默认只算 .gguf、按文件名排序；给 get 也传 --any/--filter/--sort/--limit 时，按该视图编号。",
		"Indexes use the same view as list: .gguf only, sorted by name, unless get is given --any/--filter/--sort/--limit, in which case that view is used.")
}
