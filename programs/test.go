package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/yeenasour/distrual/util/event"
)

type Node struct {
	ID int
}

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

func (n *Node) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = 2 * args.X
	return nil
}

func main() {
	var id int
	flag.IntVar(&id, "id", -1, "Assigned ID")
	flag.Parse()
	node := Node{ID: id}
	node.server()
	go node.readCommands()
	for {
		time.Sleep(time.Millisecond * 1000)
	}
}

func (n *Node) readCommands() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		fmt.Printf("Called with command %s\n", input)
	}
}

func (n *Node) server() {
	rpc.Register(n)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("listen error:", err)
	}
	m := event.Event{
		Type:    event.Init,
		NodeID:  n.ID,
		Payload: l.Addr().String(),
	}
	event.WriteEvent(m)
	go http.Serve(l, nil)
}
