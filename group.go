package job

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

/**
!!! 有收集结果 无等待时长 !!! 无此配置，收集结果必须设置等待时长
有收集结果 有等待时长  处理超时
无等待结果 无等待时长  异步执行
无收集结果  有等待时长  等待
*/

type Result struct {
	Value interface{}
	Error error
}

type GroupResult struct {
	Results []Result
	Error   error
}

// Tasker 定义任务接口
type Tasker interface {
	Execute() (interface{}, error)
}

type TaskTimeout interface {
	TimeoutHandler(ret interface{}, err error)
}

type TaskFunc func() (interface{}, error)

func (f TaskFunc) Execute() (interface{}, error) {
	return f()
}

type Logger interface {
	Info(message string, data map[string]interface{})
	Error(message string, err error, data map[string]interface{})
}

type defaultLog struct{}

func (d *defaultLog) Info(message string, data map[string]interface{}) {
	fmt.Println(message, data)
}

func (d *defaultLog) Error(message string, err error, data map[string]interface{}) {
	fmt.Println(message, err, data)
}

type Option interface {
	bind(*options)
}

type options struct {
	Log        Logger
	Duration   time.Duration
	CollectRet bool
	Ctx        context.Context
}

type logOption struct {
	Log Logger
}

func (l logOption) bind(o *options) {
	o.Log = l.Log
}

type durationOption time.Duration

func (d durationOption) bind(o *options) {
	o.Duration = time.Duration(d)
}

type collectRetOption bool

func (c collectRetOption) bind(o *options) {
	o.CollectRet = bool(c)
}

type ctxOption struct {
	ctx context.Context
}

func (c ctxOption) bind(o *options) {
	o.Ctx = c.ctx
}

func WithLog(log Logger) Option {
	return logOption{
		Log: log,
	}
}

func WithDuration(d time.Duration) Option {
	return durationOption(d)
}

func WithCtx(ctx context.Context) Option {
	return ctxOption{
		ctx: ctx,
	}
}

func WithCollectRet() Option {
	return collectRetOption(true)
}

// NewTaskGroup 创建一个新的任务组
func NewTaskGroup(name string, opts ...Option) *Group {
	defaultOptions := options{
		Log:        &defaultLog{},
		Duration:   0,
		CollectRet: false,
	}

	for _, opt := range opts {
		opt.bind(&defaultOptions)
	}

	tg := &Group{
		name:          name,
		tasks:         make([]Tasker, 0),
		timeout:       defaultOptions.Duration,
		collectResult: defaultOptions.CollectRet,
		log:           defaultOptions.Log,
		ctx:           defaultOptions.Ctx,
	}

	return tg
}

// Group 任务组结构体
type Group struct {
	mu sync.Mutex
	wg sync.WaitGroup

	name          string
	tasks         []Tasker
	timeout       time.Duration
	collectResult bool
	log           Logger
	ctx           context.Context
}

func (tg *Group) AddTask(t Tasker) {
	tg.AddTasks([]Tasker{t})
}

func (tg *Group) AddTasks(tasks []Tasker) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.tasks = append(tg.tasks, tasks...)
}

func (tg *Group) AddTaskFunc(fn TaskFunc) {
	tg.AddTasks([]Tasker{fn})
}

func (tg *Group) Reset() {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	tg.tasks = nil
	tg.ctx = nil
}

func (tg *Group) WithContext(ctx context.Context) {
	tg.ctx = ctx
}

func (tg *Group) isTimeout() bool {
	return tg.timeout > 0
}

func (tg *Group) takeContext() (context.Context, context.CancelFunc) {
	bc := tg.ctx
	if bc == nil {
		bc = context.Background()
	}

	if tg.isTimeout() {
		remain := tg.timeout
		return context.WithTimeout(bc, remain)
	} else {
		return context.WithCancel(bc)
	}
}

func (tg *Group) check() error {
	if len(tg.tasks) == 0 {
		return errors.New("no tasks to execute")
	}

	if tg.collectResult && !tg.isTimeout() {
		return errors.New("no timeout set for result collection")
	}

	return nil
}

// Execute 执行所有任务
func (tg *Group) Execute() ([]Result, error) {
	grs := <-tg.ExecChan()
	return grs.Results, grs.Error
}

func (tg *Group) ExecChan() <-chan GroupResult {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	ch := make(chan GroupResult, 1)
	if err := tg.check(); err != nil {
		ch <- GroupResult{Error: err}
		return ch
	}

	ctx, cancel := tg.takeContext() // 不主动取消
	retChan := make(chan Result, len(tg.tasks))
	tg.wg.Add(len(tg.tasks))

	tg.run(ctx, retChan)

	done := make(chan struct{})
	go func() {
		tg.wg.Wait()
		close(done)
	}()

	go func() {
		defer close(ch)
		defer cancel()

		if !tg.isTimeout() {
			cancel()
		}

		results := tg.collectResults(ctx, retChan, done)
		ch <- GroupResult{Results: results}
	}()

	return ch
}

// collectResults 收集结果
func (tg *Group) collectResults(ctx context.Context, retChan chan Result, done chan struct{}) []Result {
	// 等待所有任务完成或超时
	select {
	case <-ctx.Done():
	case <-done:
	}
	close(retChan)
	if !tg.collectResult {
		return nil
	}
	results := make([]Result, 0, cap(retChan))
	for result := range retChan {
		results = append(results, result)
	}
	return results
}

func (tg *Group) run(ctx context.Context, retChan chan Result) {
	// 启动所有任务
	for i, task := range tg.tasks {
		go func(t Tasker, i int) {
			defer tg.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					if line := bytes.IndexByte(stack[:], '\n'); line >= 0 {
						stack = stack[line+1:]
					}

					panicErr := errors.New(fmt.Sprintf("%v", r))
					tg.log.Error("task run error", panicErr, map[string]interface{}{
						"name":  tg.name,
						"i":     i,
						"stack": string(stack),
					})
				}
			}()

			value, err := t.Execute()

			ret := Result{Value: value, Error: err}
			select {
			case <-ctx.Done(): // 超时了走超时处理,  优先检查超时，因为 resultChan 有缓存，可能两个同时就绪
				if out, ok := t.(TaskTimeout); ok {
					out.TimeoutHandler(ret.Value, ret.Error)
				}
			default:
				select {
				case retChan <- ret: // 未超时正常输出
				case <-ctx.Done():
					if out, ok := t.(TaskTimeout); ok {
						out.TimeoutHandler(ret.Value, ret.Error)
					}
				}
			}
		}(task, i)
	}
}
