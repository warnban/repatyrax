// Command hashpass generates a bcrypt hash for ADMIN_PASSWORD_HASH.
// Usage: go run ./cmd/hashpass/main.go "your-secure-password"
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./cmd/hashpass/main.go \"password\"")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), 12)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
