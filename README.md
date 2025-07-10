# Job Group - 并发任务执行库

[![Go Reference](https://pkg.go.dev/badge/github.com/leyi-lee/job.svg)](https://pkg.go.dev/github.com/leyi-lee/job)
[![Go Report Card](https://goreportcard.com/badge/github.com/leyi-lee/job)](https://goreportcard.com/report/github.com/leyi-lee/job)

`job` 是一个 Go 语言库，提供了灵活且强大的方式来并发执行多个任务，支持超时控制、结果收集和错误处理功能。

## 特性

- 以受控方式并发执行多个任务
- 可配置的任务执行超时时间
- 可选的已完成任务结果收集
- 优雅的超时处理回调机制
- 单个任务的崩溃恢复
- 基于通道的异步执行模式
- 支持上下文传播取消和截止时间

## 安装

```bash
go get github.com/yourusername/job
```

## 快速开始

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/yourusername/job"
)

func main() {
    // 创建一个任务组，设置2秒超时和结果收集
    group := job.NewTaskGroup("my-tasks", 
        job.WithDuration(2*time.Second), 
        job.WithCollectRet(),
    )
    
    // 向组中添加任务
    group.AddTaskFunc(func() (interface{}, error) {
        time.Sleep(1 * time.Second)
        return "任务1已完成", nil
    })
    
    group.AddTaskFunc(func() (interface{}, error) {
        time.Sleep(3 * time.Second) // 这个任务会超时
        return "任务2已完成", nil
    })
    
    // 执行任务并收集结果
    results, err := group.Execute()
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    
    // 打印结果
    for _, result := range results {
        fmt.Printf("结果: %v\n", result)
    }
}
```

## 执行模式

该库根据配置支持四种执行模式：

1. **有收集结果 有等待时长** (`WithCollectRet()` + `WithDuration()`)
    - 等待任务完成，直到超时
    - 收集已完成任务的结果
    - 对超时任务执行超时处理器

2. **有收集结果 无等待时长** (不支持)
    - 收集结果必须设置等待时长

3. **无收集结果 有等待时长** (仅 `WithDuration()`)
    - 等待任务完成，直到超时
    - 不收集任何结果
    - 对超时任务执行超时处理器

4. **无收集结果 无等待时长** (默认)
    - 异步执行任务，不等待
    - 不收集任何结果
    - 没有超时处理

## 任务接口

实现 `Tasker` 接口来创建自定义任务：

```go
type Tasker interface {
    Execute() (interface{}, error)
}
```

如需超时处理，请实现 `TaskTimeout` 接口：

```go
type TaskTimeout interface {
    TimeoutHandler(ret interface{}, err error)
}
```

## 示例
[test 单元测试](group_test.go)

## 任务组选项

| 选项 | 描述 |
|--------|-------------|
| `WithDuration(d time.Duration)` | 设置任务的最大执行时间 |
| `WithCollectRet()` | 启用任务结果收集 |
| `WithCtx(ctx context.Context)` | 设置任务执行的父上下文 |
| `WithLog(log Logger)` | 提供自定义日志实现 |

## 最佳实践

1. **收集结果时必须设置超时**
    - 使用 `WithCollectRet()` 时必须同时使用 `WithDuration()`

2. **为长时间运行的任务实现超时处理器**
    - 实现 `TaskTimeout` 接口以优雅地处理超时

3. **在超时处理器中考虑资源清理**
    - 当任务超时时，使用超时处理器清理资源

4. **使用通道进行异步结果处理**
    - `ExecChan()` 提供了一种非阻塞方式来执行任务

## 许可证

MIT 许可证 - 详见 ([https://www.apache.org/licenses/LICENSE-2.0.html](https://www.apache.org/licenses/LICENSE-2.0.html)) 文件。