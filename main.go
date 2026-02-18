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

type Node struct {
	id     int
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	wg     sync.WaitGroup
}

type Hub struct {
	nodes map[int]*Node
}

func HubInit() *Hub {
	hub := Hub{
		nodes: make(map[int]*Node),
	}
	return &hub
}

func (h *Hub) AddNode(n *Node) {
	for i := 0; i < 20; i++ {
		if _, exists := h.nodes[i]; !exists {
			n.id = i
			h.nodes[i] = n
			return
		}
	}
}

// Probably want to sanitize the path by forcing non-PATH executables from being executed on linux
// Force explicit paths
func (h *Hub) StartNode(program string, args ...string) error {

	if len(h.nodes) >= 20 {
		return fmt.Errorf("Node limit (20) reached")
	}

	cmd := exec.Command(program, args...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	n := Node{
		id:     -1, // Assigned by hub in AddNode
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	h.AddNode(&n)

	n.wg.Add(2)
	go n.ScanPipe(n.stdout)
	go n.ScanPipe(n.stderr)
	go h.Wait(&n)

	return nil
}

func (h *Hub) RemoveNode(nodeID int) error {
	n, exists := h.nodes[nodeID]
	if !exists {
		fmt.Printf("No node with ID %d exists", nodeID)
		return nil
	}
	n.Kill()
	return nil
}

func (n *Node) ScanPipe(pipe io.ReadCloser) { // Will need refactor to pass output to parent through a channel
	defer n.wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func (h *Hub) Wait(n *Node) {
	ID := n.id
	err := n.cmd.Wait()
	n.wg.Wait()

	if err != nil {
		fmt.Printf("Node %d exited abnormally: %v\n", ID, err)
	} else {
		fmt.Printf("Node %d exited normally\n", ID)
	}

	delete(h.nodes, ID)
}

func (n *Node) Kill() {
	if n.cmd.Process == nil {
		return
	}
	n.cmd.Process.Kill()
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
			if !h.IsValidNodeID(from) || !h.IsValidNodeID(to) {
				fmt.Println("Argument \"from\" or \"to\" outside valid range")
				continue
			}
			fmt.Fprintf(h.nodes[from].stdin, "ExampleRPC to %d\n", to)
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
			err := h.StartNode(command[1], command[2:]...)
			if err != nil {
				fmt.Printf("Failed to start process, %s\n", err)
			}
		case "kill":
			if clen != 2 {
				fmt.Println("Wrong number of arguments")
				continue
			}
			nodeID, _ := strconv.Atoi(command[1])
			h.RemoveNode(nodeID)
		case "list":
			str := ""
			for k := range h.nodes {
				str += strconv.Itoa(k) + " "
			}
			str += "\n"
			fmt.Println(str)
		case "exit":
			h.ReapNodes()
			break loop
		}
	}
}

func (h *Hub) IsValidNodeID(id int) bool {
	_, exists := h.nodes[id]
	return exists
}

func (h *Hub) ReapNodes() {
	for _, node := range h.nodes {
		node.cmd.Process.Kill()
	}
}

func main() {
	hub := HubInit()
	hub.CmdLine()
}
