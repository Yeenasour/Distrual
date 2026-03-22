package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/yeenasour/distrual/util/event"
)

type Node struct {
	id     int
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	wg     sync.WaitGroup
}

func (n *Node) ScanStdout(eventChannel chan event.Event) {
	defer n.wg.Done()
	scanner := bufio.NewScanner(n.stdout)
	for scanner.Scan() {
		e, _ := event.DecodeEvent(scanner.Bytes())
		eventChannel <- *e
	}
}

func (n *Node) ScanStderr() {
	defer n.wg.Done()
	scanner := bufio.NewScanner(n.stderr)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func (n *Node) Kill() {
	if n.cmd.Process != nil {
		n.cmd.Process.Kill()
	}
}
