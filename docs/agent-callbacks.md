# Agent 回调功能

## 功能概述

为 agent 命令添加三个回调参数，允许在任务完成或失败时执行自定义 shell 命令。

## 参数说明

| 参数 | 说明 | 执行时机 |
|------|------|---------|
| `--on-complete` | 成功完成时执行的命令 | 仅当 agent 成功完成时 |
| `--on-error` | 失败时执行的命令 | 仅当 agent 失败/出错时 |
| `--on-finish` | 统一回调命令 | 无论成功/失败都执行 |

**执行规则：**
- 先根据状态执行 `--on-complete` 或 `--on-error`（如果设置）
- 然后无论成功/失败都执行 `--on-finish`（如果设置）
- 可以同时使用 `--on-complete`/`--on-error` 和 `--on-finish` 实现累加执行

## 环境变量

回调命令可访问以下环境变量：

| 变量 | 说明 | 示例 |
|------|------|------|
| `AGENT_STATUS` | 状态: `success` / `error` | `success` |
| `AGENT_EXIT_CODE` | 退出码: `0` / `1` | `0` |
| `AGENT_PROMPT` | 原始目标提示词 | `修复所有 linter 错误` |
| `AGENT_ITERATIONS` | 成功迭代次数 | `5` |
| `AGENT_TOTAL_COST` | 总成本 (USD) | `1.234` |
| `AGENT_DURATION` | 总耗时 | `1h23m45s` |
| `AGENT_ERROR` | 错误信息 (仅失败时) | `连续发生 3 次错误` |
| `AGENT_MODEL` | 使用的模型 (如果指定) | `gpt-4` |
| `AGENT_WORKTREE` | Worktree 名称 (如果使用) | `instance-1` |
| `AGENT_CWD` | 当前工作目录 | `/path/to/repo` |

## 使用示例

### 1. 基础通知

```bash
# 成功时发送桌面通知
zzz agent -p "修复 linter 错误" -m 5 \
  --on-complete 'notify-send "Agent 完成" "成功迭代: $AGENT_ITERATIONS 次"'

# 失败时记录日志
zzz agent -p "重构模块" --max-cost 10 \
  --on-error 'echo "$(date) - 失败: $AGENT_ERROR" >> agent-errors.log'
```

### 2. 统一回调

```bash
# 无论成功/失败都记录结果
zzz agent -p "添加测试" -m 3 \
  --on-finish 'echo "状态: $AGENT_STATUS, 成本: \$${AGENT_TOTAL_COST}, 时长: $AGENT_DURATION" >> agent-history.log'
```

### 3. 复杂脚本

```bash
# 调用外部脚本处理结果
zzz agent -p "性能优化" --max-duration 1h \
  --on-finish '/path/to/report-agent-result.sh'
```

示例脚本 `/path/to/report-agent-result.sh`:
```bash
#!/bin/bash
if [ "$AGENT_STATUS" = "success" ]; then
    echo "✅ Agent 成功完成"
    echo "- 迭代次数: $AGENT_ITERATIONS"
    echo "- 总成本: \$${AGENT_TOTAL_COST}"
    echo "- 耗时: $AGENT_DURATION"

    # 发送成功通知
    curl -X POST https://api.example.com/notify \
        -H "Content-Type: application/json" \
        -d "{\"status\":\"success\",\"cost\":$AGENT_TOTAL_COST}"
else
    echo "❌ Agent 执行失败"
    echo "- 错误: $AGENT_ERROR"
    echo "- 已完成迭代: $AGENT_ITERATIONS"

    # 发送失败告警
    curl -X POST https://api.example.com/alert \
        -H "Content-Type: application/json" \
        -d "{\"status\":\"error\",\"message\":\"$AGENT_ERROR\"}"
fi
```

### 4. Webhook 集成

```bash
# 成功后触发 CI/CD
zzz agent -p "更新依赖" -m 5 \
  --on-complete 'curl -X POST https://ci.example.com/trigger-build?repo=myproject'

# 发送 Slack 通知
zzz agent -p "代码优化" --max-cost 5 \
  --on-finish 'curl -X POST https://hooks.slack.com/services/XXX \
    -d "{\"text\":\"Agent 完成: $AGENT_STATUS (成本: \$${AGENT_TOTAL_COST})\"}"'
```

### 5. 条件清理

```bash
# 成功时清理临时文件
zzz agent -p "生成报告" -m 3 \
  --on-complete 'rm -rf /tmp/agent-cache/*' \
  --on-error 'echo "失败，保留临时文件用于调试"'
```

### 6. 数据记录

```bash
# 记录到数据库
zzz agent -p "自动化任务" --max-cost 10 \
  --on-finish 'psql -d metrics -c "INSERT INTO agent_runs (status, cost, duration, prompt) VALUES ('"'"'$AGENT_STATUS'"'"', $AGENT_TOTAL_COST, '"'"'$AGENT_DURATION'"'"', '"'"'$AGENT_PROMPT'"'"')"'
```

### 7. 邮件通知

```bash
# 失败时发送邮件
zzz agent -p "批量处理" -m 10 \
  --on-error 'echo "Agent 失败: $AGENT_ERROR" | mail -s "Agent 执行失败" admin@example.com'
```

## 测试回调

使用测试脚本验证回调功能：

```bash
# 创建测试脚本
cat > /tmp/test-callback.sh << 'EOF'
#!/bin/bash
echo "=== Agent Callback Test ==="
echo "Status: $AGENT_STATUS"
echo "Exit Code: $AGENT_EXIT_CODE"
echo "Iterations: $AGENT_ITERATIONS"
echo "Cost: \$${AGENT_TOTAL_COST}"
echo "Duration: $AGENT_DURATION"
echo "Prompt: $AGENT_PROMPT"
[ -n "$AGENT_ERROR" ] && echo "Error: $AGENT_ERROR"
echo "==========================="
EOF
chmod +x /tmp/test-callback.sh

# 测试成功回调（假设有一个简单任务能成功完成）
zzz agent -p "echo test" -m 1 --on-complete '/tmp/test-callback.sh'

# 测试失败回调（设置不可能完成的条件触发错误）
zzz agent -p "impossible task" --max-runs 1 --on-error '/tmp/test-callback.sh'

# 测试统一回调
zzz agent -p "test task" -m 1 --on-finish '/tmp/test-callback.sh'
```

## 注意事项

### 基础功能
1. **命令执行环境**：回调命令通过 `sh -c` 执行，支持管道、重定向等 shell 特性
2. **错误处理**：回调执行失败不影响 agent 主流程的返回值，只记录警告日志
3. **超时保护**：每个回调命令最长执行 5 分钟，超时后自动终止
4. **平台兼容性**：当前使用 `sh -c` 执行，仅适用于 Unix-like 系统 (Linux/macOS)
5. **退出码**：回调命令的退出码不影响 agent 的最终退出码

### 安全警告

⚠️ **重要**：回调命令以当前用户权限执行，具有完整的系统访问权限。

- **仅在可信环境中使用用户提供的回调命令**
- 环境变量会自动转义换行符，但仍需谨慎使用 `eval` 或 `source` 命令
- 避免在回调命令中执行未经验证的外部输入

### 并发使用

✅ **已支持**：v1.1.0 起支持多个 agent 实例并发使用回调功能（通过 `--worktree` 并发运行）。每个回调使用独立的环境变量，不会相互干扰。

```bash
# 并发运行示例
zzz agent -p "task1" -m 1 --worktree w1 --on-complete 'echo "Task 1: $AGENT_PROMPT"' &
zzz agent -p "task2" -m 1 --worktree w2 --on-complete 'echo "Task 2: $AGENT_PROMPT"' &
wait
```

## 实现细节

### 文件修改

- `app/agent/types.go`: 添加 `OnComplete`、`OnError`、`OnFinish` 字段到 `Context` 结构体
- `cmd/agent.go`: 添加命令行参数 `--on-complete`、`--on-error`、`--on-finish`
- `app/agent/callback.go`: 实现回调逻辑（新文件）
- `app/agent/agent.go`: 在 `Run()` 函数中集成回调执行

### 执行流程

```
agent.Run()
  ├─ ValidateRequirements()
  ├─ SetupWorktree()
  ├─ runMainLoop()  ← 核心执行逻辑
  ├─ ExecuteCallbacks(err)  ← 执行回调
  │   ├─ 先执行状态特定回调：on-complete 或 on-error
  │   └─ 然后执行通用回调：on-finish（如果设置）
  └─ return err
```

## 改进历史

- v1.1.0 (当前版本):
  - 修复并发安全问题：使用独立环境变量列表
  - 添加超时保护（5 分钟）
  - 改进错误处理：分离超时和执行错误
  - 调整 OnFinish 语义：累加执行而非排他
  - 环境变量转义：防止 shell 解析问题
  - 统一日志格式
- v1.0.0: 初始实现，支持 `--on-complete`、`--on-error`、`--on-finish` 三个回调参数
