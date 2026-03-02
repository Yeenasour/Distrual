package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
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

func (n *Node) ScanStdout(eventChannel chan event.Event) {
	defer n.wg.Done()
	scanner := bufio.NewScanner(n.stdout)
	for scanner.Scan() {
		e, _ := event.DecodeEvent(scanner.Bytes())
		eventChannel <- *e
	}
}

func (n *Node) AttachProxy(nodeAddr string) (net.Listener, error) {
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
				if errors.Is(err, net.ErrClosed) {
					return
				}
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

func (n *Node) Kill() {
	if n.cmd.Process != nil {
		n.cmd.Process.Kill()
	}
	if n.ln != nil {
		n.ln.Close()
	}
}
