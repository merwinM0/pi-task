# pi-task

[中文](README.md) | [English](README.en.md)

独立的 **Go 任务进度监看器**：Pi 提交工作与进度，你在另一个终端输入 `pi-task`，即可选择最近 5 次工作并实时查看。

程序只记录和展示已提交状态，**不执行任务、不访问模型、不启动后台服务**，不修改 Pi 界面。查看与刷新本身不消耗模型 token。

## 快速开始

需要 Go 1.23+、make 和 Unix 终端，主要面向 Linux、SSH 与 fish。

```fish
cd ~/mon/my_programs/pi-task
make install
fish_add_path --universal --prepend ~/.local/bin
pi-task
```

默认安装到 `~/.local/bin/pi-task`，同时安装 fish 补全，无需 sudo。更新安装后，重新打开查看器即可使用新功能。

## 键盘操作

| 按键 | 工作列表 | 进度页 |
| --- | --- | --- |
| W / S、↑ / ↓ | 上下选择 | 逐行滚动 |
| A / D、← / → | 快速移动 | 向上 / 向下翻页 |
| Enter | 查看选中工作 | — |
| Esc | 退出 | 返回列表 |
| Q / Ctrl+C | 退出 | 退出 |

字母键支持大小写；PageUp / PageDown、Home / End 仍然可用。无终端时自动输出静态文本，也可使用 `pi-task status --once` 或 `--json`。

## 与 Pi 配合

将 [任务上报约定](docs/AGENTS.example.md) 合并到项目或全局的 `AGENTS.md`，保留原有指令。Pi 通过 `start / submit / update / add / finish` 提交状态，不需要插件；写入时明确指定工作 ID，人工查看时无需记 ID。

模型可能忘记更新，因此面板展示的是最后一次提交的状态，不是独立验收结果。程序不会自动给未完成步骤打勾，也不会擅自判定长时间未更新的工作失败。

## 文档与验证

- [完整用法、fish 示例、存储与卸载](docs/USAGE.md)（中文）
- [项目进度与范围](READMAP.md)
- [维护约定与目录结构](Agent.md)

```fish
make check
make build
python3 -B tests/pty_smoke.py ./bin/pi-task
```

状态默认保存在 `~/.local/state/pi-task/`，可由 `PI_TASK_HOME` 指定另一绝对路径。查看远程进度时，请在同一服务器、同一用户与相同状态目录下运行。
