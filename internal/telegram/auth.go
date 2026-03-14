package telegram

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrMissingCode = errors.New("missing login code for first authentication")

func ReadCodeFromStdin() (string, error) {
	fmt.Print("Enter Telegram login code: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ErrMissingCode
	}
	return code, nil
}
