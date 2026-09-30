// Command adminpass turns a password into the bcrypt hash that
// ADMIN_PASSWORD_HASH wants.
//
// It reads the password from standard input rather than from an argument,
// because an argument is visible in `ps` and in shell history to anyone on the
// machine. Keep the echo off with the shell's own reader:
//
//	read -rs PW && printf '%s' "$PW" | go run ./cmd/adminpass; unset PW
//
// Then put the printed line in the server's environment, alongside
// ADMIN_USERNAME. See README.md, "Admin console".
//
// Reading stdin also means no terminal-handling dependency: an earlier version
// used golang.org/x/term for a hidden prompt, and pulling it in dragged the
// module's Go directive past the version the Dockerfile builds with.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// cost matches what the rest of the server uses for account passwords, so the
// console is not the weakest hash in the system.
const cost = 12

// minLen is a floor, not a policy. This password is the whole of what stands
// between the internet and an endpoint that deletes accounts, and unlike a
// player's password there is no email recovery behind it.
const minLen = 12

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: reading stdin:", err)
		os.Exit(1)
	}
	// Only a trailing newline is stripped. Leading and inner whitespace are
	// part of the password — a passphrase with spaces in it is a good
	// passphrase, and silently trimming would make the hash not match what
	// the operator thinks they chose.
	password := strings.TrimRight(string(raw), "\r\n")

	if len([]rune(password)) < minLen {
		fmt.Fprintf(os.Stderr, "error: use at least %d characters — this is the console's only key\n", minLen)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// The hash on stdout by itself, so it can be piped or captured; the
	// explanation on stderr, where it will not end up in the variable.
	fmt.Fprintln(os.Stderr, "Set these in the server's environment:")
	fmt.Fprintln(os.Stderr, "  ADMIN_USERNAME=<the name you sign in with>")
	fmt.Fprintln(os.Stderr, "  ADMIN_PASSWORD_HASH=<the line below>")
	fmt.Fprintln(os.Stderr, "The password itself is stored nowhere — keep it in a password manager.")
	fmt.Println(string(hash))
}
