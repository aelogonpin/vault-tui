package vault

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// promptString reads a single line from stdin, echoed normally. Used for
// values like usernames that aren't sensitive.
func promptString(label string) (string, error) {
	fmt.Print(label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptPassword reads a single line from stdin without echoing it back,
// falling back to a normal (visible) read if stdin isn't a terminal (e.g.
// piped input in scripts/tests).
func promptPassword(label string) (string, error) {
	fmt.Print(label)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return promptStringNoLabel()
	}
	b, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func promptStringNoLabel() (string, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
