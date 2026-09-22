// 小工具：生成 bcrypt 密码哈希，用于构造压测账号
// 用法: go run ./tools/seed/pwdhash <密码>
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: pwdhash <密码>")
		os.Exit(1)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(hashed))
}
