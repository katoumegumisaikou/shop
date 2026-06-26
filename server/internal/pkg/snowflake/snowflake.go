package snowflake

import (
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once
)

// Init 初始化雪花节点，workerID 范围 0-1023。
func Init(workerID int64) {
	once.Do(func() {
		var err error
		node, err = snowflake.NewNode(workerID)
		if err != nil {
			panic(err)
		}
	})
}

// NextID 生成雪花 ID
func NextID() int64 {
	if node == nil {
		panic("snowflake node is not initialized; call snowflake.Init before NextID")
	}
	return node.Generate().Int64()
}
