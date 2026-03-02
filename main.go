package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/yeenasour/distrual/util/event"
)

type Node struct {
	id     int
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	ln     net.Listener
	wg     sync.WaitGroup
}

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

func (n *Node) ScanStdout(eventChannel chan event.Event) {
	defer n.wg.Done()
	scanner := bufio.NewScanner(n.stdout)
	for scanner.Scan() {
		e, _ := event.DecodeEvent(scanner.Bytes())
		eventChannel <- *e
	}
}

func (n *Node) attachProxy(nodeAddr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Println("Proxy error:", err)
		return nil, err
	}

	n.ln = ln

	go func() {
		for {
			clientConn, err := ln.Accept()
			if err != nil {
				log.Println("Incomming connection error:", err)
				continue
			}
			go forwardConn(clientConn, nodeAddr)
		}
	}()

	return ln, nil
}

func forwardConn(clientConn net.Conn, endpoint string) {
	defer clientConn.Close()

	nodeConn, err := net.Dial("tcp", endpoint)
	if err != err {
		log.Println("Couldn't connect to endpoint")
		return
	}
	defer nodeConn.Close()

	io.Copy(nodeConn, clientConn)
	io.Copy(clientConn, nodeConn)
}

func (n *Node) ScanStderr() {
	defer n.wg.Done()
	scanner := bufio.NewScanner(n.stderr)
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

	delete(h.proxies, ID)
	delete(h.nodes, ID)
}

func (n *Node) Kill() {
	if n.cmd.Process != nil {
		n.cmd.Process.Kill()
	}
	if n.ln != nil {
		n.ln.Close()
	}
}

func (h *Hub) EventHandler() {
	for e := range h.eventChannel {
		switch e.Type {
		case event.Init:
			fmt.Printf("Node %d initialized at port %s\n", e.NodeID, e.Payload)
			ln, _ := h.nodes[e.NodeID].attachProxy(e.Payload.(string))
			h.proxies[e.NodeID] = ln.Addr().String()
		}
	}
}

func (h *Hub) PrintProxies() {
	fmt.Print("[ ")
	for _, p := range h.proxies {
		fmt.Printf("%s ", p)
	}
	fmt.Println("]")
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
			for k := range h.nodes {
				str += strconv.Itoa(k) + " "
			}
			str += "\n"
			fmt.Println(str)
		case "proxies":
			h.PrintProxies()
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
