package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/MoeForge/moejs"
)

// ctx.process：插件完全信任，可以执行本机命令（与主程序同权限，见 docs/plugin-system.md §7.6）。
// 宿主不做沙箱；让 AI 调用命令的插件应自己用 ctx.ui.confirm 让用户逐条确认。

const (
	processDefaultExecTimeout = 60 * time.Second
	processMaxOutputBytes     = 8 << 20 // exec 每路输出最多保留 8 MiB
)

type processOptions struct {
	Cwd      string            `json:"cwd"`
	Env      map[string]string `json:"env"`
	Timeout  *float64          `json:"timeout"` // 毫秒；exec 默认 60 秒，0 表示不限
	Input    string            `json:"input"`
	Shell    bool              `json:"shell"`
	Encoding string            `json:"encoding"` // auto（默认）| utf8 | gbk | big5 | shift_jis | euc-kr
}

type execResult struct {
	Code      int    `json:"code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	TimedOut  bool   `json:"timedOut"`
	Truncated bool   `json:"truncated"`
}

// buildCommand 组装命令：shell 为 true 时 command 是一整行命令，交给系统 shell（Windows cmd.exe，其他平台 /bin/sh）。
func buildCommand(ctx context.Context, command string, args []string, opts processOptions) (*exec.Cmd, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("命令不能为空")
	}
	var cmd *exec.Cmd
	if opts.Shell {
		line := command
		if len(args) > 0 {
			line += " " + strings.Join(args, " ")
		}
		cmd = shellCommand(ctx, line)
	} else {
		cmd = exec.CommandContext(ctx, command, args...)
	}
	cmd.Dir = strings.TrimSpace(opts.Cwd)
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			if strings.TrimSpace(k) != "" {
				env = append(env, k+"="+v)
			}
		}
		cmd.Env = env
	}
	configureProcessAttrs(cmd)
	cmd.Cancel = func() error { return killProcessTree(cmd) }
	cmd.WaitDelay = 2 * time.Second
	return cmd, nil
}

// cappedBuffer 只保留前 limit 字节，记录是否截断。
type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
			b.truncated = true
		} else {
			b.buf.Write(p)
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func runExec(parent context.Context, command string, args []string, opts processOptions) (execResult, error) {
	timeout := processDefaultExecTimeout
	if opts.Timeout != nil {
		timeout = time.Duration(*opts.Timeout) * time.Millisecond
	}
	ctx, cancel := parent, context.CancelFunc(func() {})
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()

	cmd, err := buildCommand(ctx, command, args, opts)
	if err != nil {
		return execResult{}, err
	}
	stdout := &cappedBuffer{limit: processMaxOutputBytes}
	stderr := &cappedBuffer{limit: processMaxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if opts.Input != "" {
		cmd.Stdin = strings.NewReader(opts.Input)
	}
	runErr := cmd.Run()
	res := execResult{
		Code:      -1,
		Stdout:    decodeProcessOutput(stdout.buf.Bytes(), opts.Encoding),
		Stderr:    decodeProcessOutput(stderr.buf.Bytes(), opts.Encoding),
		TimedOut:  errors.Is(ctx.Err(), context.DeadlineExceeded),
		Truncated: stdout.truncated || stderr.truncated,
	}
	if cmd.ProcessState != nil {
		res.Code = cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) && !res.TimedOut {
		// 没能启动（找不到命令等）才算异常；非零退出码与超时都正常返回结果
		if cmd.ProcessState == nil {
			return res, runErr
		}
	}
	return res, nil
}

// decodeProcessOutput 把命令输出转成字符串：auto 时合法 UTF-8 原样返回，否则按系统代码页解码（Windows 控制台程序常输出 GBK 等）。
func decodeProcessOutput(data []byte, encoding string) string {
	enc := strings.ToLower(strings.TrimSpace(encoding))
	if enc == "" || enc == "auto" {
		if utf8.Valid(data) {
			return string(data)
		}
		enc = systemLegacyEncoding()
	}
	if enc == "utf8" || enc == "utf-8" || enc == "" {
		return strings.ToValidUTF8(string(data), "�")
	}
	if out, ok := decodeLegacy(data, enc); ok {
		return out
	}
	return strings.ToValidUTF8(string(data), "�")
}

// ─────────────────────────── spawn ───────────────────────────

type spawnedProcess struct {
	p       *Plugin
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	exited  bool
	stdout  moejs.Value
	stderr  moejs.Value
	onExit  moejs.Value
	pending map[string][]string // 回调注册之前到达的输出，按 stdout / stderr 分开缓存
	exit    map[string]any      // 进程已退出但 onExit 还没注册时暂存
}

// 以下方法只在插件线程上调用。

func (sp *spawnedProcess) callback(stream string) moejs.Value {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	switch stream {
	case "stdout":
		return sp.stdout
	case "stderr":
		return sp.stderr
	default:
		return sp.onExit
	}
}

func (sp *spawnedProcess) call(rt *moejs.Runtime, fn moejs.Value, arg any) {
	v, err := toJSValue(rt, arg)
	if err != nil {
		return
	}
	if _, err := rt.CallFunction(fn, moejs.Undefined(), v); err != nil {
		sp.p.recordError(err)
	}
}

// deliver 把输出投递到插件线程；还没注册回调时先缓存。
func (sp *spawnedProcess) deliver(stream, text string) {
	sp.p.Post(func(rt *moejs.Runtime) {
		fn := sp.callback(stream)
		if !fn.IsObject() {
			sp.mu.Lock()
			sp.pending[stream] = append(sp.pending[stream], text)
			sp.mu.Unlock()
			return
		}
		sp.call(rt, fn, text)
	})
}

// setCallback 注册回调，并补发注册之前缓存的输出或退出结果。
func (sp *spawnedProcess) setCallback(rt *moejs.Runtime, stream string, fn moejs.Value) {
	sp.mu.Lock()
	switch stream {
	case "stdout":
		sp.stdout = fn
	case "stderr":
		sp.stderr = fn
	default:
		sp.onExit = fn
	}
	pending := sp.pending[stream]
	delete(sp.pending, stream)
	exit := sp.exit
	if stream == "exit" {
		sp.exit = nil
	}
	sp.mu.Unlock()
	for _, text := range pending {
		sp.call(rt, fn, text)
	}
	if stream == "exit" && exit != nil {
		sp.call(rt, fn, exit)
	}
}

func (sp *spawnedProcess) pump(r io.Reader, stream, encoding string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 32*1024)
	var carry []byte
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := append(carry, buf[:n]...)
			carry = nil
			// 尽量在换行处切分，避免把一个多字节字符拆到两次回调里
			if err == nil {
				if i := bytes.LastIndexByte(chunk, '\n'); i >= 0 && i < len(chunk)-1 {
					carry = append([]byte(nil), chunk[i+1:]...)
					chunk = chunk[:i+1]
				} else if i < 0 && len(chunk) < 4096 {
					// 没有换行的短输出（如命令行提示）直接交付，但先留下末尾不完整的 UTF-8 字符
					if cut := incompleteUTF8Tail(chunk); cut > 0 {
						carry = append([]byte(nil), chunk[len(chunk)-cut:]...)
						chunk = chunk[:len(chunk)-cut]
					}
				}
			}
			if len(chunk) > 0 {
				sp.deliver(stream, decodeProcessOutput(chunk, encoding))
			}
		}
		if err != nil {
			if len(carry) > 0 {
				sp.deliver(stream, decodeProcessOutput(carry, encoding))
			}
			return
		}
	}
}

// incompleteUTF8Tail 返回末尾不完整 UTF-8 序列的字节数。
func incompleteUTF8Tail(b []byte) int {
	for i := 1; i <= 3 && i <= len(b); i++ {
		c := b[len(b)-i]
		if c&0xC0 == 0x80 {
			continue // 续字节，继续往前找首字节
		}
		if c >= 0xC0 {
			need := 2
			if c >= 0xF0 {
				need = 4
			} else if c >= 0xE0 {
				need = 3
			}
			if need > i {
				return i
			}
		}
		return 0
	}
	return 0
}

func (sp *spawnedProcess) kill() {
	sp.mu.Lock()
	exited := sp.exited
	sp.mu.Unlock()
	if !exited && sp.cmd.Process != nil {
		_ = killProcessTree(sp.cmd)
	}
}

func (p *Plugin) spawnProcess(r *moejs.Realm, command string, args []string, opts processOptions) (moejs.Value, error) {
	ctx, cancel := context.WithCancel(p.scope.Context())
	cmd, err := buildCommand(ctx, command, args, opts)
	if err != nil {
		cancel()
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return moejs.Undefined(), throwJSError(r, fmt.Sprintf("启动进程失败: %v", err))
	}
	sp := &spawnedProcess{p: p, cmd: cmd, stdin: stdin, pending: make(map[string][]string)}
	// 插件卸载时结束进程树
	dispose := p.scope.Effect(func() {
		sp.kill()
		cancel()
	})
	if opts.Input != "" {
		_, _ = io.WriteString(stdin, opts.Input)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go sp.pump(stdoutPipe, "stdout", opts.Encoding, &wg)
	go sp.pump(stderrPipe, "stderr", opts.Encoding, &wg)
	go func() {
		wg.Wait()
		waitErr := cmd.Wait()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		sp.mu.Lock()
		sp.exited = true
		sp.mu.Unlock()
		cancel()
		dispose()
		errMsg := ""
		var exitErr *exec.ExitError
		if waitErr != nil && !errors.As(waitErr, &exitErr) {
			errMsg = waitErr.Error()
		}
		payload := map[string]any{"code": code}
		if errMsg != "" {
			payload["error"] = errMsg
		}
		p.Post(func(rt *moejs.Runtime) {
			fn := sp.callback("exit")
			if !fn.IsObject() {
				sp.mu.Lock()
				sp.exit = payload
				sp.mu.Unlock()
				return
			}
			sp.call(rt, fn, payload)
		})
	}()

	setCallback := func(name, stream string) moejs.NativeFunc {
		return p.safeNative("process."+name, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, name+" 需要函数参数")
			}
			sp.setCallback(p.rt, stream, fn)
			return moejs.Undefined(), nil
		})
	}
	handle := map[string]any{
		"pid":      cmd.Process.Pid,
		"onStdout": setCallback("onStdout", "stdout"),
		"onStderr": setCallback("onStderr", "stderr"),
		"onExit":   setCallback("onExit", "exit"),
		"write": p.safeNative("process.write", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if _, err := io.WriteString(sp.stdin, argString(args, 0)); err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			return moejs.Undefined(), nil
		}),
		"closeStdin": p.safeNative("process.closeStdin", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			_ = sp.stdin.Close()
			return moejs.Undefined(), nil
		}),
		"kill": p.safeNative("process.kill", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			sp.kill()
			return moejs.Undefined(), nil
		}),
	}
	return p.rt.FromGo(handle)
}

// parseProcessArgs 解析 (command, args?, opts?)；第二个参数是对象时视为 opts。
func (p *Plugin) parseProcessArgs(args []moejs.Value) (string, []string, processOptions, error) {
	command := argString(args, 0)
	var list []string
	var opts processOptions
	optsIdx := 2
	if a := moejs.Arg(args, 1); a.IsObject() {
		raw, err := p.rt.AppendJSON(nil, a)
		if err != nil {
			return "", nil, opts, err
		}
		if len(raw) > 0 && raw[0] == '[' {
			if err := json.Unmarshal(raw, &list); err != nil {
				return "", nil, opts, fmt.Errorf("args 必须是字符串数组: %w", err)
			}
		} else {
			optsIdx = 1
		}
	}
	if o := moejs.Arg(args, optsIdx); o.IsObject() {
		raw, err := p.rt.AppendJSON(nil, o)
		if err != nil {
			return "", nil, opts, err
		}
		if err := json.Unmarshal(raw, &opts); err != nil {
			return "", nil, opts, fmt.Errorf("进程选项格式错误: %w", err)
		}
	}
	return command, list, opts, nil
}

func (p *Plugin) buildProcessAPI() map[string]any {
	return map[string]any{
		"platform": runtime.GOOS,
		"exec": p.safeNative("process.exec", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			command, list, opts, err := p.parseProcessArgs(args)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			promise, settle := p.loop.NewPromise()
			go func() {
				res, err := runExec(p.scope.Context(), command, list, opts)
				settle(func(rt *moejs.Runtime) (moejs.Value, error) {
					if err != nil {
						return moejs.Undefined(), throwJSError(rt.Realm(), err.Error())
					}
					return toJSValue(rt, res)
				})
			}()
			return promise, nil
		}),
		"spawn": p.safeNative("process.spawn", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			command, list, opts, err := p.parseProcessArgs(args)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			return p.spawnProcess(r, command, list, opts)
		}),
	}
}
