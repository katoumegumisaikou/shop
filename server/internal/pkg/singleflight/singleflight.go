package singleflight

import (
	"fmt"
	"sync"
)

var syncMap sync.Map

type Result[T any] struct {
	wg   sync.WaitGroup
	data T
	err  error
}

type Func[T any] func() (T, error)

func Lock[T any](key string, f Func[T]) (t T, err error) {
	// 检查是否已经存在
	result := &Result[T]{}
	result.wg.Add(1)                              // 添加1，避免其他人已经获取到result时未加1
	typeKey := fmt.Sprintf("%T:%s", *new(T), key) // 让key带上类型
	if r, exist := syncMap.LoadOrStore(typeKey, result); !exist {
		// defer 保证 panic 时也能放行等待者并清 key
		defer func() {
			result.wg.Done()
			syncMap.Delete(typeKey) // 移除key，只保证短时间只有一个请求
		}()
		// 恢复程序，避免整个服务挂掉，并且让执行者和等待者的返回一致
		defer func() {
			if ret := recover(); ret != nil {
				result.err = fmt.Errorf("函数中发生了panic %v", ret)
				err = result.err
			}
		}()
		// 实际函数执行者
		result.data, result.err = f()
		return result.data, result.err
	} else {
		var ok bool
		result, ok = r.(*Result[T])
		if !ok {
			return *new(T), fmt.Errorf("类型转化失败")
		}
		result.wg.Wait()
		return result.data, result.err
	}
}
