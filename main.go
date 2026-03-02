package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yeenasour/distrual/util/event"
)

type Hub struct {
	nodes        map[int]*Node
	proxies      map[int]string
	eventChannel chan event.Event
	nextID       int
	programDir   string
}

func HubInit() *Hub {
	hub := Hub{
		nodes:        make(map[int]*Node),
		proxies:      make(map[int]string),
		eventChannel: make(chan event.Event, 10),
		nextID:       0,
		programDir:   ProgramDir(),
	}
	return &hub
}

func (h *Hub) AddNode(id int, n *Node) {
	h.nodes[id] = n
}

func (h *Hub) generateID() int {
	prev := h.nextID
	h.nextID++
	return prev
}

// Probably want to sanitize the path by forcing non-PATH executables from being executed on linux
// Force explicit paths
func (h *Hub) StartNode(program string, args ...string) error {

	if len(h.nodes) >= 20 {
		return fmt.Errorf("Node limit (20) reached")
	}

	nodeID := h.generateID()

	idArg := fmt.Sprintf("--id=%d", nodeID)
	argsWithID := append([]string{idArg}, args...)

	execPath := filepath.Join(h.programDir, program)

	cmd := exec.Command(execPath, argsWithID...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	n := Node{
		id:     nodeID,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	h.AddNode(nodeID, &n)

	n.wg.Add(2)
	go n.ScanStdout(h.eventChannel)
	go n.ScanStderr()
	go h.Wait(&n)

	return nil
}

func (h *Hub) RemoveNode(nodeID int) error {
	n, exists := h.nodes[nodeID]
	if !exists {
		fmt.Printf("No node with ID %d exists\n", nodeID)
		return nil
	}
	n.Kill()
	return nil
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

	delete(h.proxies, ID)
	delete(h.nodes, ID)
}

func (h *Hub) EventHandler() {
	for e := range h.eventChannel {
		switch e.Type {
		case event.Init:
			fmt.Printf("Node %d initialized at port %s\n", e.NodeID, e.Payload)
			ln, _ := h.nodes[e.NodeID].AttachProxy(e.Payload.(string))
			h.proxies[e.NodeID] = ln.Addr().String()
		}
	}
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
		case "killall":
			h.ReapNodes()
		case "list":
			str := ""
			for i, n := range h.nodes {
				str += fmt.Sprintf("(%d - {%s, %s} )\n", i, n.ln.Addr().String(), h.proxies[i])
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

func ProgramDir() string {
	rootDir, _ := os.Getwd()
	/*exePath, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get executable path:", err)
	}
	rootDir := filepath.Dir(exePath)*/
	path := filepath.Join(rootDir, "programs")
	return path
}

func main() {
	hub := HubInit()
	go hub.EventHandler()
	hub.CmdLine()
}
