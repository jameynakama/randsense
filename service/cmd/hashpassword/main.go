// Command hashpassword prints a bcrypt hash of a password typed at the
// terminal, for ADMIN_PASSWORD_HASH.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	fmt.Fprint(os.Stderr, "Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("read password: %v", err)
	}
	if len(pw) == 0 {
		log.Fatal("password is empty")
	}
	hash, err := bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	fmt.Println(string(hash))
}
