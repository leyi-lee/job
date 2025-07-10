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

### 基本任务执行

```go
func Example_basic() {
    group := job.NewTaskGroup("basic-tasks")
    
    group.AddTaskFunc(func() (interface{}, error) {
        // 执行某些操作
        return "结果", nil
    })
    
    // 执行后不等待（异步执行）
    _, _ = group.Execute()
}
```

### 带超时的执行

```go
func Example_timeout() {
    group := job.NewTaskGroup("timeout-tasks", job.WithDuration(5*time.Second))
    
    group.AddTask(&customTask{name: "task-1"})
    
    // 带超时执行
    _, _ = group.Execute()
    
    // 等待可能的超时处理器完成
    time.Sleep(1 * time.Second)
}

type customTask struct {
    name string
}

func (t *customTask) Execute() (interface{}, error) {
    // 长时间运行的操作
    time.Sleep(10 * time.Second)
    return t.name + " 已完成", nil
}

func (t *customTask) TimeoutHandler(ret interface{}, err error) {
    fmt.Printf("任务 %s 超时\n", t.name)
}
```

### 收集结果

```go
func Example_collectResults() {
    group := job.NewTaskGroup("collect-tasks", 
        job.WithDuration(2*time.Second),
        job.WithCollectRet(),
    )
    
    group.AddTaskFunc(func() (interface{}, error) {
        return "快速任务", nil
    })
    
    group.AddTaskFunc(func() (interface{}, error) {
        time.Sleep(3 * time.Second) // 这个任务会超时
        return "慢速任务", nil
    })
    
    results, _ := group.Execute()
    
    for _, result := range results {
        fmt.Printf("结果: %v\n", result)
    }
    // 输出: 结果: 快速任务
}
```

### 使用通道进行异步执行

```go
func Example_channels() {
    group := job.NewTaskGroup("channel-tasks", 
        job.WithDuration(1*time.Second),
        job.WithCollectRet(),
    )
    
    for i := 0; i < 5; i++ {
        i := i // 捕获循环变量
        group.AddTaskFunc(func() (interface{}, error) {
            time.Sleep(time.Duration(i) * 300 * time.Millisecond)
            return fmt.Sprintf("任务 %d", i), nil
        })
    }
    
    // 获取结果通道
    resultChan := group.ExecChan()
    
    // 异步处理结果
    go func() {
        groupResult := <-resultChan
        if groupResult.Error != nil {
            fmt.Printf("错误: %v\n", groupResult.Error)
            return
        }
        
        for _, result := range groupResult.Results {
            if result.Error == nil {
                fmt.Printf("获得: %v\n", result.Value)
            }
        }
    }()
    
    // 这里可以做其他工作...
    time.Sleep(2 * time.Second)
}
```

### 自定义上下文

```go
func Example_context() {
    // 创建带取消功能的父上下文
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    group := job.NewTaskGroup("context-tasks", 
        job.WithCtx(ctx),
        job.WithDuration(10*time.Second),
    )
    
    group.AddTaskFunc(func() (interface{}, error) {
        // 这个任务将被父上下文取消
        select {
        case <-time.After(5 * time.Second):
            return "已完成", nil
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    })
    
    // 开始执行
    go func() {
        _, _ = group.Execute()
    }()
    
    // 2秒后取消执行
    time.Sleep(2 * time.Second)
    cancel()
}
```

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