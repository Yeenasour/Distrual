package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Child struct {
	id     int
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	wg     sync.WaitGroup
}

type ChildExit struct {
	id  int
	err error
}

type Hub struct {
	children map[int]*Child
}

func HubInit() *Hub {
	hub := Hub{
		children: make(map[int]*Child),
	}
	return &hub
}

func (h *Hub) AddChild(c *Child) {
	for i := 0; i < 20; i++ {
		if _, exists := h.children[i]; !exists {
			c.id = i
			h.children[i] = c
			return
		}
	}
}

// Probably want to sanitize the path by forcing non-PATH executables from being executed on linux
// Force explicit paths
func (h *Hub) StartChild(program string, args ...string) error {

	if len(h.children) >= 20 {
		return fmt.Errorf("Child limit (20) reached")
	}

	cmd := exec.Command(program, args...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	c := Child{
		id:     -1, // Assigned by hub in AddChild
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	h.AddChild(&c)

	c.wg.Add(2)
	go c.ScanPipe(c.stdout)
	go c.ScanPipe(c.stderr)
	go h.Wait(&c)

	return nil
}

func (h *Hub) RemoveChild(childID int) error {
	c, exists := h.children[childID]
	if !exists {
		fmt.Printf("No child with ID %d exists", childID)
		return nil
	}
	c.Kill()
	return nil
}

func (c *Child) ScanPipe(pipe io.ReadCloser) { // Will need refactor to pass output to parent through a channel
	defer c.wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func (h *Hub) Wait(c *Child) {
	ID := c.id
	err := c.cmd.Wait()
	c.wg.Wait()

	if err != nil {
		fmt.Printf("Child %d exited abnormally: %v\n", ID, err)
	} else {
		fmt.Printf("Child %d exited normally\n", ID)
	}

	delete(h.children, ID)
}

func (c *Child) Kill() {
	if c.cmd.Process == nil {
		return
	}
	c.cmd.Process.Kill()
}

func (h *Hub) CmdLine() {
	reader := bufio.NewReader(os.Stdin)
loop:
	for {
		fmt.Printf("> ")
		input, _ := reader.ReadString('\n')
		command := strings.Split(strings.TrimSpace(input), " ")
		clen := len(command)
		if clen == 0 {
			continue
		}
		switch command[0] {
		case "example":
			if clen != 3 {
				fmt.Println("Wrong number of arguments")
				continue
			}
			from, _ := strconv.Atoi(command[1])
			to, _ := strconv.Atoi(command[2])
			if !h.IsValidChildID(from) || !h.IsValidChildID(to) {
				fmt.Println("Argument \"from\" or \"to\" outside valid range")
				continue
			}
			fmt.Fprintf(h.children[from].stdin, "ExampleRPC to %d\n", to)
		case "create":
			if clen < 2 {
				fmt.Println("Must provide at least a binary to run")
				continue
			}
			fmt.Printf("Executable: %s - ", command[1])
			fmt.Printf("Arguments: ")
			for _, arg := range command[2:] {
				fmt.Printf("%s ", arg)
			}
			fmt.Printf("\n")
			err := h.StartChild(command[1], command[2:]...)
			if err != nil {
				fmt.Printf("Failed to start process, %s\n", err)
			}
		case "kill":
			if clen != 2 {
				fmt.Println("Wrong number of arguments")
				continue
			}
			childID, _ := strconv.Atoi(command[1])
			h.RemoveChild(childID)
		case "list":
			str := ""
			for k := range h.children {
				str += strconv.Itoa(k) + " "
			}
			str += "\n"
			fmt.Println(str)
		case "exit":
			h.ReapChildren()
			break loop
		}
	}
}

func (h *Hub) IsValidChildID(id int) bool {
	_, exists := h.children[id]
	return exists
}

func (h *Hub) ReapChildren() {
	for _, child := range h.children {
		child.cmd.Process.Kill()
	}
}

func main() {
	hub := HubInit()
	hub.CmdLine()
}
