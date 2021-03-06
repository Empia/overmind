package start

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DarthSim/overmind/term"
	"github.com/DarthSim/overmind/utils"
)

const runningCheckInterval = 100 * time.Millisecond

type process struct {
	command   string
	root      string
	sessionID string
	output    *multiOutput
	conn      *processConnection
	proc      *os.Process

	Name  string
	Color int
}

type processesMap map[string]*process

func newProcess(name, sessionID string, color int, command, root string, output *multiOutput) *process {
	return &process{
		command:   command,
		root:      root,
		sessionID: sessionID,
		output:    output,
		Name:      name,
		Color:     color,
	}
}

func (p *process) WindowID() string {
	return fmt.Sprintf("%v:%v", p.sessionID, p.Name)
}

func (p *process) Start(socketPath string, newSession bool) (err error) {
	if p.Running() {
		return
	}

	args := []string{
		"-n", p.Name, "-P", "-F", "#{pane_pid}",
		"-c", p.root, os.Args[0], "launch", p.Name, p.command, socketPath,
		"\\;", "allow-rename", "off",
	}

	if newSession {
		ws, err := term.GetSize(os.Stdout)
		if err != nil {
			return err
		}

		args = append([]string{"new", "-d", "-s", p.sessionID, "-x", strconv.Itoa(int(ws.Cols)), "-y", strconv.Itoa(int(ws.Rows))}, args...)
	} else {
		args = append([]string{"neww", "-t", p.sessionID}, args...)
	}

	var pid string
	var ipid int

	if pid, err = utils.RunCmdOutput("tmux", args...); err == nil {
		if ipid, err = strconv.Atoi(strings.TrimSpace(pid)); err == nil {
			p.proc, err = os.FindProcess(ipid)
		}
	}

	err = utils.ConvertError(err)

	return
}

func (p *process) Pid() int {
	return p.proc.Pid
}

func (p *process) Wait() {
	for _ = range time.Tick(runningCheckInterval) {
		if !p.Running() {
			return
		}
	}
}

func (p *process) Running() bool {
	return p.proc != nil && p.proc.Signal(syscall.Signal(0)) == nil
}

func (p *process) Stop() {
	if p.Running() && p.conn != nil {
		p.conn.Stop()
	}
}

func (p *process) Kill() {
	if p.Running() {
		p.output.WriteBoldLine(p, []byte("Killing..."))
		p.proc.Signal(syscall.SIGKILL)
	}
}

func (p *process) Restart() {
	if p.conn != nil {
		p.conn.Restart()
	}
}

func (p *process) AttachConnection(conn net.Conn) {
	if p.conn == nil {
		p.conn = &processConnection{conn}
		go p.scanConn()
	}
}

func (p *process) scanConn() {
	err := utils.ScanLines(p.conn.Reader(), func(b []byte) bool {
		p.output.WriteLine(p, b)
		return true
	})
	if err != nil {
		p.output.WriteErr(p, fmt.Errorf("Connection error: %v", err))
	}
}
