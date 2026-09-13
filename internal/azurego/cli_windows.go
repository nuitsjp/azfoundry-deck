package azurego

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func runAzureCLICommand(ctx context.Context, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, "az", args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	// The Windows Azure CLI launcher runs Python in isolated mode. Its pipes
	// use the system ANSI code page, not the console code page or PYTHONUTF8.
	// Decode before JSON parsing: encoding/json silently replaces invalid UTF-8.
	codePage := windows.GetACP()
	out, outErr := decodeCLIOutput(stdout.Bytes(), codePage)
	errOut, errOutErr := decodeCLIOutput(stderr.Bytes(), codePage)
	return out, errOut, errors.Join(runErr, outErr, errOutErr)
}

func decodeCLIOutput(data []byte, codePage uint32) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	const mbErrInvalidChars = 0x00000008
	length, err := windows.MultiByteToWideChar(codePage, mbErrInvalidChars, &data[0], int32(len(data)), nil, 0)
	if err != nil {
		return nil, fmt.Errorf("Azure CLI の出力をコードページ %d から UTF-8 に変換できません: %w", codePage, err)
	}
	wide := make([]uint16, length)
	length, err = windows.MultiByteToWideChar(codePage, mbErrInvalidChars, &data[0], int32(len(data)), &wide[0], int32(len(wide)))
	if err != nil {
		return nil, fmt.Errorf("Azure CLI の出力をコードページ %d から UTF-8 に変換できません: %w", codePage, err)
	}
	return []byte(string(utf16.Decode(wide[:length]))), nil
}
