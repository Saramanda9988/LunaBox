package main

import (
	"fmt"
	"time"
)

// main 模拟一个游戏进程
// 运行 2 秒后自动退出
func main() {
	fmt.Println("Test game started...")
	fmt.Println("Sleeping for 2 seconds to simulate gameplay...")
	time.Sleep(2 * time.Second)
	fmt.Println("Test game finished.")
}
