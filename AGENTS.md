# pi-task 项目维护规范

- 这是只记录、展示调用方提交状态的 Go CLI，不是任务执行器或调度器。
- 不读取 Pi 的私有会话格式，不引入模型、API 密钥、网络服务或常驻后台进程。
- 默认 `pi-task` / `pi-task status` 列出最近更新的 5 次工作，W/S、上下方向键选择，A/D、左右方向键翻页，Enter 进入实时查看。查看器不得修改任务状态。
- 写入必须有显式工作 ID；禁止用“最近一条”隐式指定写入目标。
- 保留文件锁、原子快照、私有权限与文本控制字符防护；不得因显示需求执行提交内容。
- `finish` 不得自动把未完成步骤改为完成；取消、失败也不能伪装成成功。
- 维护中文界面、fish 可直接复制的使用示例；fish 不支持 Bash heredoc。
- 更改后运行 `make check`；涉及终端行为时再执行 `make build` 和 `python3 -B tests/pty_smoke.py ./bin/pi-task`。
- 真终端验证使用临时 `PI_TASK_HOME`，不要在用户真实历史里创建测试记录。
- 给其他项目使用的代理指令示例位于 `docs/AGENTS.example.md`，不要未经授权覆盖用户全局指令。

## 目录与开发流程

- `cmd/pi-task/`：命令入口；`internal/cli/`：参数解析与命令；`internal/task/`：状态规则。
- `internal/store/`：私有快照与并发写入；`internal/ui/`：只读终端界面和键盘解析。
- `completions/`：fish 补全；`tests/pty_smoke.py`：真实终端回归。
- `README.md`、`README.en.md`：同步维护的中英文简介；`docs/USAGE.md`：详细用法；`READMAP.md`：已实现能力与明确待办。
- 项目指令统一维护在 `AGENTS.md`，由 Pi 自动加载，不另建不同大小写、单复数变体或兼容链接。
- 修改前先阅读上述文档与相关实现；修改后同步帮助、快捷键提示、文档和测试。
- 使用 gofmt 格式化 Go 文件；执行 `make check`。终端行为修改另执行 `make build`、`python3 -B tests/pty_smoke.py ./bin/pi-task`；安装更新使用 `make install`，让用户重新打开查看器。
- 只提交本次相关修改，使用准确的中文 Git 提交说明，不自行创建 Git 仓库。
