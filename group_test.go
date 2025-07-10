package job

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"
	"time"
)

/*!!! 有收集结果 无等待时长 !!! 无此配置，收集结果必须设置等待时长
有收集结果 有等待时长  处理超时
无等待结果 无等待时长  异步执行
无收集结果  有等待时长  等待*/

func newTestSt(name string, duration time.Duration, execTimeout bool) *test_st {
	return &test_st{
		name:        name,
		duration:    duration,
		execTimeout: execTimeout,
	}
}

type test_st struct {
	name        string
	duration    time.Duration
	execTimeout bool
	subTask     bool
}

func (s *test_st) Execute() (interface{}, error) {
	if s.duration > 0 {
		time.Sleep(s.duration)
	}
	fmt.Println("执行", s.name, takeCurFormatTime())

	if s.subTask {
		newGroupName := fmt.Sprintf("%s_sub_task", s.name)
		tg := NewTaskGroup(newGroupName, WithCollectRet(), WithDuration(s.duration))
		tg.AddTask(newTestSt(fmt.Sprintf("%s_%s", newGroupName, "task1"), 0, true))
		tg.AddTask(newTestSt(fmt.Sprintf("%s_%s", newGroupName, "task2"), time.Second*5, true))
		fmt.Println(newGroupName, "开始执行", takeCurFormatTime())
		ret, err := tg.Execute()
		fmt.Println(newGroupName, "执行完成", takeCurFormatTime())
		if err != nil {
			fmt.Println(newGroupName, "执行失败", takeCurFormatTime())
			return nil, err
		}
		return ret, err
	}

	return s.name, nil
}

func (s *test_st) TimeoutHandler(ret interface{}, err error) {
	if !s.execTimeout {
		return
	}
	fmt.Println(s.name, "超时处理", ret, err, takeCurFormatTime())
}

func (s *test_st) withSubTask() *test_st {
	s.subTask = true
	return s
}

// TestResultTimeout 有等待时长，且收集结果
func TestResultTimeout(t *testing.T) {
	as := assert.New(t)
	printGoroutineNums()
	//
	tg := NewTaskGroup("result_timeout", WithCollectRet(), WithDuration(time.Second))

	// 正常完结且可以收集到结果
	tg.AddTask(newTestSt("normal", 0, true))
	tg.AddTask(newTestSt("normal2", 1, true))
	ret, err := tg.Execute()
	fmt.Print("ret:")
	printJson(ret)

	as.NoError(err)
	as.Equalf(2, len(ret), "ret %v", ret)

	tg.Reset()

	// 有超时，且收集结果，只能收集到未超时的且执行超时处理
	tg.AddTask(newTestSt("normal", 0, true))
	tg.AddTask(newTestSt("timeout", 2*time.Second, true))
	tg.AddTask(newTestSt("timeout2", 3*time.Second, true))
	ret, err = tg.Execute()
	fmt.Print("ret:")
	printJson(ret)
	as.NoError(err)
	as.Equalf(1, len(ret), "ret %v", ret)

	tg.Reset()

	// 全部超时
	tg.AddTask(newTestSt("timeout1", 2*time.Second, true))
	tg.AddTask(newTestSt("timeout2", 3*time.Second, true))
	ret, err = tg.Execute()
	fmt.Print("ret:")
	printJson(ret)

	as.NoError(err)
	as.Equalf(0, len(ret), "ret %v", ret)

	time.Sleep(15 * time.Second)

}

// TestNoResultNotTime, 异步执行完，所以无超时处理
func TestNoResultNotTime(t *testing.T) {
	as := assert.New(t)

	printGoroutineNums()

	tg := NewTaskGroup("no_result_timeout")

	tg.AddTask(newTestSt("timeout1", 2*time.Second, false))
	tg.AddTask(newTestSt("timeout2", 3*time.Second, true))
	ret, err := tg.Execute()
	fmt.Print("ret:")
	printJson(ret)

	as.NoError(err)
	time.Sleep(15 * time.Second)

}

func TestTimeoutNoResult(t *testing.T) {
	as := assert.New(t)
	printGoroutineNums()

	tg := NewTaskGroup("timeout_no_result", WithDuration(time.Second))

	tg.AddTask(newTestSt("normal", 0, true))
	tg.AddTask(newTestSt("timeout1", 2*time.Second, false))
	tg.AddTask(newTestSt("timeout2", 3*time.Second, true))
	ret, err := tg.Execute()
	fmt.Print("ret:")
	printJson(ret)

	as.NoError(err)
	as.Nil(ret)

	time.Sleep(15 * time.Second)
}

func TestResultNoTimeout(t *testing.T) {
	as := assert.New(t)
	printGoroutineNums()

	tg := NewTaskGroup("timeout_no_result", WithCollectRet())
	tg.AddTask(newTestSt("normal", 0, true))
	ret, err := tg.Execute()
	fmt.Println("ret: err", ret, err)
	as.Error(err) // 有错误是对的
	as.Nil(ret)

	time.Sleep(15 * time.Second)
}

// TestEmbed 测试嵌套
func TestEmbed(t *testing.T) {
	as := assert.New(t)
	printGoroutineNums()
	tg := NewTaskGroup("grand", WithDuration(time.Second))
	tg.AddTask(newTestSt("grand1", 1, true).withSubTask())
	tg.AddTask(newTestSt("grand2", 2*time.Second, true).withSubTask())
	fmt.Println("grand 开始执行", takeCurFormatTime())
	ret, err := tg.Execute()
	fmt.Println("grand 执行完成", takeCurFormatTime())
	as.NoError(err)
	as.Nil(ret)
	//
	//go func() {
	//	c := time.Tick(time.Second)
	//	for range c {
	//		fmt.Println("count: ", runtime.NumGoroutine(), takeCurFormatTime())
	//	}
	//}()

	time.Sleep(10 * time.Second)
}

func TestExecChan(t *testing.T) {
	printGoroutineNums()
	tg := NewTaskGroup("result_timeout_chan", WithCollectRet(), WithDuration(time.Second))
	// 正常完结且可以收集到结果
	tg.AddTask(newTestSt("normal", 0, true))
	tg.AddTask(newTestSt("normal2", time.Second, true))
	tg.AddTask(newTestSt("normal3", time.Second*3, true))

	retChan := tg.ExecChan()

	fmt.Println("中间任务")
	time.Sleep(5 * time.Second)

	grs := <-retChan
	fmt.Print("ret:", grs.Results)
	printJson(grs)

	time.Sleep(15 * time.Second)
}

func takeCurFormatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func printJson(ret any) {
	b, _ := json.Marshal(ret)
	fmt.Println(string(b))
}

func printGoroutineNums() {
	go func() {
		c := time.Tick(time.Second)
		for range c {
			fmt.Println("count: ", runtime.NumGoroutine(), takeCurFormatTime())
		}
	}()
}

func TestNums(t *testing.T) {
	printGoroutineNums()

	time.Sleep(10 * time.Second)
}
