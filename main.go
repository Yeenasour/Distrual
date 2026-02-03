package main

import (
	//"fmt"
	"bufio"
	"fmt"
	"io"
	"log"
	//"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	//"time"
)

type Child struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

type Hub struct {
	children []*Child
}

func HubInit() *Hub {
	hub := Hub{
		children: []*Child{},
	}
	return &hub
}

func (h *Hub) AddChild(c *Child) {
	h.children = append(h.children, c)
}

func StartChild(program string, args ...string) (*Child, error) {
	cmd := exec.Command(program, args...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	return &Child{
		cmd,
		stdin,
		stdout,
	}, nil
}

func (c *Child) attachScanner() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Child stdout error:", err)
	}
	fmt.Printf("Hello")
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
			if (from < 0 || from >= len(h.children)) || (to < 0 || to >= len(h.children)) {
				fmt.Println("Argument \"from\" or \"to\" outside valid ID-range")
			}
			fmt.Fprintf(h.children[from].stdin, "ExampleRPC to %d\n", to)
		case "exit":
			break loop
		}
	}
}

func main() {
	//fs := http.FileServer(http.Dir("./static"))
	//http.Handle("/", fs)

	//go http.ListenAndServe(":8080", nil)

	hub := HubInit()

	child1, _ := StartChild("programs/test.exe", "1234")
	child2, _ := StartChild("programs/test.exe", "1235")
	go child1.attachScanner()
	go child2.attachScanner()
	hub.AddChild(child1)
	hub.AddChild(child2)

	hub.CmdLine()
}
