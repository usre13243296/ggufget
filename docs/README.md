# 文档索引

ggufget 的文档分三层：**README 负责让人愿意试，MANUAL 负责把事说全，SPEC 负责定义行为**。改行为时先改 SPEC，再改代码，最后同步 MANUAL 与 README；用户可见的变化写进 [CHANGELOG](../CHANGELOG.md)。

## 入门

| 文档 | 内容 | 适合谁 |
| --- | --- | --- |
| [README](../README.md) | 定位、特性、安装、30 秒上手、命令与选项速查 | 第一次听说这个工具的人 |
| [MANUAL.md](../MANUAL.md) | 完整中文说明书：交互界面、镜像、选择文件、下载、校验、日志、配置、排障 | 已经装好、准备长期用的人 |
| [MANUAL.en.md](../MANUAL.en.md) | 英文说明书，与中文版同步 | English readers |

## 行为与设计

| 文档 | 内容 |
| --- | --- |
| [SPEC.md](../SPEC.md) | 命令行契约：命令面、选项、退出码、目录布局、下载引擎、JSON 事件。**行为的唯一来源** |
| [design-principles.md](design-principles.md) | 一个好用的命令行工具应该具备什么：14 条原则与反例 |
| [verify.md](verify.md) | 下载状态机、`.part` / `.part.seg`、续传依据、校验分级、并发与限流 |
| [bench.md](bench.md) | 三种镜像的测速方法、原始数据、注意事项与端到端验证 |

## 背景与协作

| 文档 | 内容 |
| --- | --- |
| [blog.md](blog.md) | 写这个工具的来龙去脉：两次走错的路、为什么选 Go、几个设计决定 |
| [collaboration.md](collaboration.md) | 协作与诊断协议：这轮踩过的坑、分工方式、下一步计划 |
| [legacy/](legacy/) | 最早的两个 Python 脚本和 `.bat` 包装，留作对照，不再维护 |

## 维护规则

1. **行为变更**：先改 `SPEC.md`，再改代码与测试，最后同步 `MANUAL.md` / `MANUAL.en.md` / `README.md`。
2. **用户可见变更**：写进 `CHANGELOG.md` 的 `Unreleased`，发版时再归到版本号下。
3. **数字与单位**：速度统一标注单位（MiB/s 与 MB/s 换算：1 MiB/s = 1.048576 MB/s）；测速数字必须注明时间与“单机单次”。
4. **不要写进文档的**：未经实测的“最快镜像”结论、与代码不一致的默认值、只存在于计划里的功能（计划写在 `collaboration.md` 或 issue 里）。
