package main

import (
	"bufio"
	"fmt"
	"github.com/yeenasour/distrual/util/msg"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"time"
)

type Node struct {
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
	node := Node{}
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
	m := msg.Message{
		Type:    msg.Init,
		Payload: l.Addr().String(),
	}
	msg.WriteMessage(m)
	go http.Serve(l, nil)
}
