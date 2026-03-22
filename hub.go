package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yeenasour/distrual/util/commands"
	"github.com/yeenasour/distrual/util/event"
)

type Hub struct {
	nodes        map[int]*Node
	proxies      map[int]*Proxy
	eventChannel chan event.Event
	nextID       int
	programDir   string
}

func HubInit() *Hub {
	hub := Hub{
		nodes:        make(map[int]*Node),
		proxies:      make(map[int]*Proxy),
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

	h.proxies[ID].Close()

	delete(h.proxies, ID)
	delete(h.nodes, ID)
}

func (h *Hub) EventHandler() {
	for e := range h.eventChannel {
		switch e.Type {
		case event.Init:
			fmt.Printf("Node %d initialized at port %s\n", e.NodeID, e.Payload)
			p, err := AttachProxy(e.Payload.(string))
			if err != nil {
				log.Println("Malformed init message: ", err)
				h.nodes[e.NodeID].Kill()
			}
			h.proxies[e.NodeID] = p
		}
	}
}

func (h *Hub) CmdLine() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("> ")
		input, _ := reader.ReadString('\n')
		command, args, err := commands.TrimSplitCommand(input)
		if err != nil {
			fmt.Println("Command error: ", err)
			continue
		}

		handler, exists := handlers[command]
		if !exists {
			continue
		}

		res := handler(h, args)

		if res.Error != nil {
			fmt.Println("Error:", res.Error)
		}
		if res.Output != "" {
			fmt.Println(res.Output)
		}

		if res.Done {
			break
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
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get executable path:", err)
	}
	rootDir := filepath.Dir(exePath)
	path := filepath.Join(rootDir, "programs")
	return path
}

func main() {
	hub := HubInit()
	go hub.EventHandler()
	hub.CmdLine()
}
