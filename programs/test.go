package main

import (
	"bufio"
	"fmt"
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
	if len(os.Args) != 2 {
		log.Fatal("Wrong number of arguments")
	}
	//fmt.Println("Started program")
	node := Node{}
	node.server(os.Args[1])
	go node.readCommands()
	for {
		time.Sleep(time.Millisecond * 1000)
	}
}

func (n *Node) readCommands() {
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		fmt.Fprintf(writer, "Called with command %s\n", input)
		writer.Flush()
	}
}

func (n *Node) server(adress string) {
	rpc.Register(n)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":"+adress)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	go http.Serve(l, nil)
}
