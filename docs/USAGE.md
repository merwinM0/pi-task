# pi-task 完整用法

[返回中文简介](../README.md) · [English overview](../README.en.md)

独立的 **Go 任务进度监看器**。Pi 通过命令提交工作和进度，你在另一终端只需输入 `pi-task`：选择最近 5 次工作，Enter 进入实时进度页。

不修改 Pi 界面，不执行任务，不启动守护进程，不访问模型或网络。Go 1.23+；主要面向 Linux（包括远程 SSH）与 fish，Unix 终端。

## 安装

```fish
cd ~/mon/my_programs/pi-task
make install
fish_add_path --universal --prepend ~/.local/bin
pi-task --version
```

默认安装到 `~/.local/bin/pi-task`，不需要 sudo，并在安装前缀的 `share/fish/vendor_completions.d/` 安装 fish 补全。`make install PREFIX=/usr/local` 可更改安装前缀，但系统目录可能需要权限。安装时原子替换二进制，不影响已打开的查看器。

只构建不安装：`make build`，然后运行 `./bin/pi-task`。

## 查看：不需要名称或 ID

```fish
pi-task
```

`pi-task status` 效果相同。列表按**最近更新时间**排序，显示最近 5 次工作，包括已结束的工作。

| 按键 | 工作列表 | 进度页 |
| --- | --- | --- |
| W / S、↑ / ↓ | 上下选择 | 上下滚动 |
| Enter | 查看选中工作 | — |
| A / D、← / → | 快速移动 | 向上 / 向下翻页 |
| PageUp / PageDown | 快速移动（兼容） | 翻页（兼容） |
| Home / End | 第一/最后项 | 顶部/底部 |
| Esc | 退出 | 返回列表 |
| Q | 退出 | 退出 |
| Ctrl+C | 退出（状态码 130） | 退出（状态码 130） |

A/W 向上、D/S 向下：W/S 逐行移动，A/D 按半个终端高度翻页；字母键支持大小写，左右方向键也用于翻页。自动每 500 毫秒读取状态；没有变化时不重绘。刷新不会随意把选择跳到另一项。支持中文、Emoji、终端缩放和长清单滚动；窄终端使用紧凑列表。正常退出、Ctrl+C、SIGTERM、SIGHUP 会恢复终端模式与光标，SIGKILL 无法被任何程序捕获。

```text
PI TASK / 最近 5 次工作
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
› ▶ 修改登录页面 · 1/4 · 进行中
    11-07 15:00:00 · /home/mo/EngNote
  ✓ 调整终端配置 · 3/3 · 已完成
    11-07 14:12:00 · /home/mo/dotfiles
```

状态：`○ 待开始`、`▶ 进行中`、`✓ 已完成`、`! 已阻塞`、`– 已取消`、`✗ 失败`。使用透明终端背景，青色标题、浅蓝进行中、浅绿完成、暖黄阻塞、浅红失败，同时使用文字和符号，不仅依赖颜色。`NO_COLOR=1 pi-task` 禁用颜色。亮色适配现有深色终端，以不透明 `#1e1e2e` 为参考时，语义色对比度为 8.80–12.96:1；透明背景或其他主题不作统一对比度保证，可用无色模式跟随终端默认文字色。

## 提交：由 Pi 或你自己执行

每次工作有独立 ID，**查看时不需要 ID，写入时必须明确传入**。下面是可以直接在 fish 中执行的例子：

```fish
set -l run_id (pi-task start --title "修改登录页面" --task "检查实现" --task "修改布局" --task "运行测试")

pi-task update --id $run_id --task 1 --state running

# 完成上一步、开始下一步，合并成一次状态写入
pi-task update --id $run_id --task 1 --state done --next 2
pi-task update --id $run_id --task 2 --state done --next 3
pi-task update --id $run_id --task 3 --state done --note "测试全部通过"

pi-task finish --id $run_id
```

程序不会执行标题里的任何内容。start 只向 stdout 返回 ID；一般更新只返回 `ok ID rev=N`，不会反复把完整清单送回模型。

### 先创建，再提交清单

```fish
set -l run_id (pi-task start --title "排查启动问题")
pi-task submit --id $run_id --task "定位原因" --task "修复" --task "验证"
```

也可以从文件或标准输入读取 **JSON 字符串数组**：

```fish
printf '%s\n' '["检查配置", "修改实现", "运行测试"]' | pi-task start --title "修复问题" --file -
pi-task submit --id $run_id --file /path/to/plan.json
```

以上是两种不同提交方式的示例，不要重复为同一工作提交初始清单。`--file` 和重复 `--task` 不能混用；已提交的清单不能整体覆盖，避免丢失进度。

### 增补、修订与阻塞

以下操作用于尚未结束的工作，应在 `finish` 前执行：

```fish
pi-task add --id $run_id --text "补充回归测试"
pi-task update --id $run_id --task 2 --text "修订后的步骤标题"
pi-task update --id $run_id --task 2 --state blocked --note "等待访问权限"
pi-task update --id $run_id --task 2 --note ""
```

步骤状态：`pending`、`running`、`done`、`blocked`、`cancelled`。允许调用方明确报告多个并行进行中的步骤；程序不做调度。步骤 ID 从 1 开始且不会因为显示顺序变化而重排。

### 结束不是自动打勾

```fish
pi-task finish --id $run_id                    # 全部步骤都是 done 才能成功
pi-task finish --id $run_id --state failed     # 失败结束，不篡改步骤状态
pi-task finish --id $run_id --state cancelled  # 取消结束，不篡改步骤状态
```

三种结束方式择一使用。结束后记录保留、不可继续写入；查看器不会被关闭，仍可检查记录。它只是监看器，不证明实际代码正确，也不会在 Pi 退出时自动结束工作。

工作超过 5 分钟没更新，进度页会提示可能仍在执行，不会擅自判定失败。长时间构建没有状态变化是正常的。

## 与 Pi 配合

将 [任务上报约定](AGENTS.example.md) 的约定合并到相应项目的 `AGENTS.md`，或合并到 `~/.pi/agent/AGENTS.md` 让所有项目使用。**不要覆盖已有指令**。本项目没有自动修改你的全局指令。

Pi 不需要插件，使用其 Bash 工具调用 CLI 即可。每次调用显式写工作 ID，不要指望不同工具调用共享 shell 变量。简单问答不用创建任务；只在阶段变化时更新。

模型生成清单、调用状态命令和读取命令结果会消耗 token；查看器运行时长、刷新次数、存储读取不会消耗 token，也不需要模型 API。指令不能保证模型永不忘记更新，以最后提交的状态为准。

## 静态输出和脚本

```fish
pi-task status --once
pi-task status --json
pi-task status --id SOME_ID --once
pi-task status --id SOME_ID --json
```

stdin/stdout 不是终端或 `TERM=dumb` 时默认输出静态文本，不写终端控制序列。`--json` 可供脚本读取；损坏记录会通过 `warnings` 返回，而不是静默当作没有任务。`--watch` 可显式要求实时界面，但在非交互终端会报错，避免脚本意外挂住。

## 存储和并发

默认存储：`~/.local/state/pi-task/runs/ID.json`。设置 `XDG_STATE_HOME` 时使用其下的 `pi-task/`；`PI_TASK_HOME` 可以指定另一绝对路径，优先级最高。

- 目录权限 0700，快照和锁文件 0600（新创建时）；不保存聊天记录或命令输出，只保存提交的简短文本、当前目录和时间。
- 写入采用跨进程文件锁、私有临时文件、fsync 和原子重命名；失败的更新不覆盖旧快照。
- 查看器只读，不创建空状态目录，也不改变完成状态。
- 写入 ID 经过校验，禁止路径穿越；用户文本中的换行和控制字符被拒绝，显示层进一步净化终端控制字符。
- 历史不自动删除；不依赖 Pi 私有会话格式、Node 版本或 shell 配置。
- 推荐同一机器的本地文件系统，不承诺网络文件系统上的锁与刷新行为。

远程任务需要在同一台服务器、同一用户和相同状态目录下查看：另开 SSH 连接后运行 `pi-task`。程序不提供跨机器同步或公网服务。

## 验证

```fish
make check
make build
python3 -B tests/pty_smoke.py ./bin/pi-task
```

覆盖状态校验、失败不误报完成、并发写入、原子快照、权限、最近 5 条排序、损坏记录、CLI、中文与 Emoji 宽度、键盘序列、真实 PTY 的实时刷新、缩放和终端恢复。测试使用临时状态目录，不访问用户任务历史或调用模型。

## 卸载

删除 `~/.local/bin/pi-task` 和 `~/.local/share/fish/vendor_completions.d/pi-task.fish` 即可，不需要停止服务。状态记录会保留在状态目录；是否删除由你决定。
