// 小工具：用与后端一致的密钥签发 JWT，方便压测/调试需要鉴权的接口
// 用法: go run ./tools/token <user_id> [secret]   (secret 缺省用内置默认值)
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Lhh220/g-video/logic-server/pkg/utils"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: token <user_id> [secret]")
		os.Exit(1)
	}
	uid, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "user_id 必须是整数:", err)
		os.Exit(1)
	}
	if len(os.Args) >= 3 {
		utils.SetSecret(os.Args[2])
	}
	token, err := utils.GenerateToken(uid, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(token)
}
