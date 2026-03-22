package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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
loop:
	for {
		fmt.Printf("> ")
		input, _ := reader.ReadString('\n')
		command, args, err := commands.TrimSplitCommand(input)
		if err != nil {
			fmt.Println("Command error: ", err)
			continue
		}
		arglen := len(args)
		if command == "" {
			continue
		}
		switch command {
		case "ping":
			if arglen != 2 {
				fmt.Println("Wrong number of arguments")
				continue
			}
			from, _ := strconv.Atoi(args[0])
			to, _ := strconv.Atoi(args[1])
			if !h.IsValidNodeID(from) || !h.IsValidNodeID(to) {
				fmt.Println("Argument \"from\" or \"to\" outside valid range")
				continue
			}
			fmt.Fprintf(h.nodes[from].stdin, "ping %s\n", h.proxies[to].ln.Addr().String())
		case "create":
			if arglen < 1 {
				fmt.Println("Must provide at least a binary to run")
				continue
			}
			fmt.Printf("Executable: %s - ", args[0])
			fmt.Printf("Arguments: ")
			for _, arg := range args[1:] {
				fmt.Printf("%s ", arg)
			}
			fmt.Printf("\n")
			err := h.StartNode(args[0], args[1:]...)
			if err != nil {
				fmt.Printf("Failed to start process, %s\n", err)
			}
		case "kill":
			if arglen != 1 {
				fmt.Println("Wrong number of arguments")
				continue
			}
			nodeID, _ := strconv.Atoi(args[0])
			h.RemoveNode(nodeID)
		case "killall":
			h.ReapNodes()
		case "list":
			str := ""
			for i := range h.nodes {
				p := h.proxies[i]
				str += fmt.Sprintf("(%d - {n - %s. p - %s} )\n", i, p.endpoint, p.ln.Addr().String())
			}
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
