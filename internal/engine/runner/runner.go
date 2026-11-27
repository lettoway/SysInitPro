package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Exec 执行命令并返回合并输出（自动隐藏 Windows 黑框）
func Exec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	applyPlatformAttr(cmd)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	if err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// ExecPS 执行 PowerShell 脚本，强制 UTF-8 输出避免中文乱码
func ExecPS(ctx context.Context, script string) (string, error) {
	utf8Script := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + script
	return Exec(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", utf8Script)
}

// Stream 实时流式执行命令，并发读取 stdout/stderr 并回调
func Stream(ctx context.Context, logFn func(string), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	applyPlatformAttr(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	readPipe := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" && logFn != nil {
				logFn(line)
			}
		}
	}

	wg.Add(2)
	go readPipe(stdout)
	go readPipe(stderr)
	wg.Wait()
	return cmd.Wait()
}
