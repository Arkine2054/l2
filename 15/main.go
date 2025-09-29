package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// Простой unix-shell
// Поддерживает встроенные: cd, pwd, echo, kill, ps
// Поддерживает: внешние команды, пайплайны (|), редиректы (> <), подстановку $VAR,
// условные операторы && и ||, Ctrl+C - прерывает текущую команду, Ctrl+D - выход

// Структуры для команд

type Cmd struct {
	Args   []string
	In     string // filename for stdin, empty = default
	Out    string // filename for stdout, empty = default
	Append bool   // >> not required but simple flag
}

type Job struct {
	Cmds []*Cmd // pipeline commands
	Cond string // "" | "&&" | "||" - relation to previous job
}

var currentCmdProcs []*os.Process

func main() {
	reader := bufio.NewReader(os.Stdin)

	// Handle Ctrl+C: forward to running process group
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			// send SIGINT to all running cmds
			for _, p := range currentCmdProcs {
				if p == nil {
					continue
				}
				_ = p.Signal(syscall.SIGINT)
			}
		}
	}()

	for {
		cwd, _ := os.Getwd()
		fmt.Printf("%s$ ", cwd)
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return
			}
			fmt.Fprintln(os.Stderr, "read error:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		jobs, perr := parseLine(line)
		if perr != nil {
			fmt.Fprintln(os.Stderr, "parse error:", perr)
			continue
		}

		lastStatus := 0
		for i, job := range jobs {
			// Evaluate conditional based on previous exit
			if i > 0 {
				prevCond := job.Cond
				if prevCond == "&&" && lastStatus != 0 {
					continue
				}
				if prevCond == "||" && lastStatus == 0 {
					continue
				}
			}

			status := runJob(job)
			lastStatus = status
		}
	}
}

// parseLine разбивает строку на Job-ы, учитывая &&, ||
func parseLine(line string) ([]*Job, error) {
	// Сначала разделим по && и || сохраняя оператор
	var jobs []*Job
	s := strings.TrimSpace(line)
	if s == "" {
		return jobs, nil
	}

	// простой state machine: ищем && или || top-level (не внутри кавычек)
	parts := splitByLogical(s)
	for _, p := range parts {
		trimmed := strings.TrimSpace(p.text)
		if trimmed == "" {
			continue
		}
		cmds, err := parsePipeline(trimmed)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &Job{Cmds: cmds, Cond: p.cond})
	}
	return jobs, nil
}

// result for logical split
type logicalPart struct {
	text string
	cond string // operator that precedes this part relative to previous: "" for first, else "&&"/"||"
}

func splitByLogical(s string) []logicalPart {
	var res []logicalPart
	cur := ""
	cond := ""
	inS := false
	inD := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inD {
			inS = !inS
		}
		if c == '"' && !inS {
			inD = !inD
		}
		// look ahead for && or || if not in quotes
		if !inS && !inD && i+1 < len(s) {
			two := s[i : i+2]
			if two == "&&" || two == "||" {
				// push current
				res = append(res, logicalPart{text: cur, cond: cond})
				cur = ""
				cond = two
				i++ // skip next
				continue
			}
		}
		cur += string(c)
	}
	res = append(res, logicalPart{text: cur, cond: cond})
	// first item's cond must be empty
	if len(res) > 0 {
		res[0].cond = ""
	}
	return res
}

// parsePipeline: разбивает по | и парсит каждый командный элемент
func parsePipeline(s string) ([]*Cmd, error) {
	parts := splitTopLevel(s, '|')
	var cmds []*Cmd
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cmd, err := parseCmd(p)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// splitTopLevel разбивает строку по разделителю, игнорируя символы внутри кавычек
func splitTopLevel(s string, sep rune) []string {
	var res []string
	cur := strings.Builder{}
	inS := false
	inD := false
	for _, r := range s {
		if r == '\'' && !inD {
			inS = !inS
		}
		if r == '"' && !inS {
			inD = !inD
		}
		if r == sep && !inS && !inD {
			res = append(res, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	res = append(res, cur.String())
	return res
}

// parseCmd: обработка редиректов > и < и аргументов
func parseCmd(s string) (*Cmd, error) {
	tokens := tokenize(s)
	if len(tokens) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := &Cmd{}
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if t == ">" || t == ">>" {
			appendMode := t == ">>"
			i++
			if i >= len(tokens) {
				return nil, errors.New("no file after >")
			}
			cmd.Out = tokens[i]
			cmd.Append = appendMode
		} else if t == "<" {
			i++
			if i >= len(tokens) {
				return nil, errors.New("no file after <")
			}
			cmd.In = tokens[i]
		} else {
			// normal arg
			arg := expandEnv(t)
			cmd.Args = append(cmd.Args, arg)
		}
		i++
	}
	return cmd, nil
}

// tokenize: простая токенизация с поддержкой кавычек
func tokenize(s string) []string {
	var res []string
	cur := strings.Builder{}
	inS := false
	inD := false
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\'' && !inD {
			inS = !inS
			i++
			continue
		}
		if c == '"' && !inS {
			inD = !inD
			i++
			continue
		}
		if !inS && !inD && (c == ' ' || c == '\t') {
			if cur.Len() > 0 {
				res = append(res, cur.String())
				cur.Reset()
			}
			i++
			continue
		}
		// handle >, >>, < as separate tokens
		if !inS && !inD && (c == '>' || c == '<') {
			if cur.Len() > 0 {
				res = append(res, cur.String())
				cur.Reset()
			}
			if c == '>' && i+1 < len(s) && s[i+1] == '>' {
				res = append(res, ">>")
				i += 2
				continue
			}
			res = append(res, string(c))
			i++
			continue
		}
		cur.WriteByte(c)
		i++
	}
	if cur.Len() > 0 {
		res = append(res, cur.String())
	}
	return res
}

// Подстановка переменных $VAR и ${VAR}
func expandEnv(s string) string {
	// простая реализация: находим $ и берём имя до не-буквы-цифры-_ или { }
	out := strings.Builder{}
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '$' {
			if i+1 < len(s) && s[i+1] == '{' {
				j := i + 2
				for j < len(s) && s[j] != '}' {
					j++
				}
				if j < len(s) && s[j] == '}' {
					name := s[i+2 : j]
					out.WriteString(os.Getenv(name))
					i = j + 1
					continue
				}
				// no closing brace: write literally
				out.WriteByte(c)
				i++
				continue
			} else {
				j := i + 1
				for j < len(s) && (isAlnumUnderscore(s[j])) {
					j++
				}
				name := s[i+1 : j]
				out.WriteString(os.Getenv(name))
				i = j
				continue
			}
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

func isAlnumUnderscore(b byte) bool {
	if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' {
		return true
	}
	return false
}

// runJob: выполняет pipeline
func runJob(job *Job) int {
	if len(job.Cmds) == 0 {
		return 0
	}

	// Builtins short-circuit: if single cmd and builtin, run in-process
	if len(job.Cmds) == 1 && isBuiltin(job.Cmds[0].Args) {
		return runBuiltin(job.Cmds[0])
	}

	// иначе создаём внешние команды и пайпим
	n := len(job.Cmds)
	cmds := make([]*exec.Cmd, n)
	pipes := make([]*os.File, 2*(n-1))
	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "pipe error:", err)
			return 1
		}
		pipes[2*i] = r
		pipes[2*i+1] = w
	}

	// prepare exec.Cmds
	for i, c := range job.Cmds {
		if len(c.Args) == 0 {
			continue
		}
		cmds[i] = exec.Command(c.Args[0], c.Args[1:]...)
		// set process group so we can signal it
		cmds[i].SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		// stdin
		if c.In != "" {
			f, err := os.Open(c.In)
			if err != nil {
				fmt.Fprintln(os.Stderr, "open infile:", err)
				return 1
			}
			cmds[i].Stdin = f
		} else if i > 0 {
			cmds[i].Stdin = pipes[2*(i-1)]
		}
		// stdout
		if c.Out != "" {
			flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
			if c.Append {
				flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
			}
			f, err := os.OpenFile(c.Out, flag, 0644)
			if err != nil {
				fmt.Fprintln(os.Stderr, "open outfile:", err)
				return 1
			}
			cmds[i].Stdout = f
		} else if i < n-1 {
			cmds[i].Stdout = pipes[2*i+1]
		} else {
			cmds[i].Stdout = os.Stdout
		}
		// stderr
		cmds[i].Stderr = os.Stderr
	}

	// Start each command
	currentCmdProcs = []*os.Process{}
	for i, c := range cmds {
		if c == nil {
			continue
		}
		if err := c.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "start error:", err)
			// close pipes
			for _, p := range pipes {
				if p != nil {
					p.Close()
				}
			}
			return 1
		}
		// record process
		currentCmdProcs = append(currentCmdProcs, c.Process)
		// close writer ends in parent where appropriate
		if i > 0 {
			// close read end in parent? keep for next
		}
	}

	// Close pipe fds in parent
	for _, p := range pipes {
		if p != nil {
			p.Close()
		}
	}

	// Wait for cmds
	status := 0
	for _, c := range cmds {
		if c == nil {
			continue
		}
		err := c.Wait()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				if ws, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					status = ws.ExitStatus()
				}
			} else {
				fmt.Fprintln(os.Stderr, "wait error:", err)
				status = 1
			}
		} else {
			// success
			status = 0
		}
	}
	// clear current procs
	currentCmdProcs = []*os.Process{}
	return status
}

// isBuiltin: проверяет имя команды
func isBuiltin(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "cd", "pwd", "echo", "kill", "ps":
		return true
	}
	return false
}

func runBuiltin(cmd *Cmd) int {
	if len(cmd.Args) == 0 {
		return 0
	}
	switch cmd.Args[0] {
	case "cd":
		if len(cmd.Args) < 2 {
			home := os.Getenv("HOME")
			if home == "" {
				home = "/"
			}
			if err := os.Chdir(home); err != nil {
				fmt.Fprintln(os.Stderr, "cd:", err)
				return 1
			}
			return 0
		}
		path := cmd.Args[1]
		if err := os.Chdir(path); err != nil {
			fmt.Fprintln(os.Stderr, "cd:", err)
			return 1
		}
		return 0
	case "pwd":
		wd, _ := os.Getwd()
		fmt.Println(wd)
		return 0
	case "echo":
		args := cmd.Args[1:]
		fmt.Println(strings.Join(args, " "))
		return 0
	case "kill":
		if len(cmd.Args) < 2 {
			fmt.Fprintln(os.Stderr, "kill: pid required")
			return 1
		}
		p, err := strconv.Atoi(cmd.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "kill: bad pid")
			return 1
		}
		if err := syscall.Kill(p, syscall.SIGTERM); err != nil {
			fmt.Fprintln(os.Stderr, "kill:", err)
			return 1
		}
		return 0
	case "ps":
		// простая реализация через системную утилиту ps
		c := exec.Command("ps", "-aux")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "ps:", err)
			return 1
		}
		return 0
	}
	return 0
}

// Тесты: не включены — запускайте программу, вводите команды

// Примечания / ограничения:
// - Запись и чтение с файлов открываются с простыми флагами.
// - Парсер простой и не покрывает все edge-cases (например вложенные кавычки + переменные внутри).
// - Для корректной передачи Ctrl+C дочерним процессам используется Setpgid, и сигнал пересылается текущим процессам.
// - Пайплайн не реализует оптимизацию для полностью встроенных команд в середине пайплайна.
// - Условные операторы &&/|| применяются между блоками (job'ами) разделёнными этими операторами.
