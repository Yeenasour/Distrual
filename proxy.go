package main

import (
	"errors"
	"io"
	"log"
	"net"
)

type Proxy struct {
	ln       net.Listener
	endpoint string
}

func AttachProxy(endpoint string) (*Proxy, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Println("Proxy error:", err)
		return nil, err
	}

	p := &Proxy{
		ln:       ln,
		endpoint: endpoint,
	}

	go p.accept()
	return p, nil
}

func (p *Proxy) accept() {
	for {
		clientConn, err := p.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Println("Incomming connection error:", err)
			continue
		}
		go p.forwardConn(clientConn)
	}
}

func (p *Proxy) forwardConn(clientConn net.Conn) {
	defer clientConn.Close()

	nodeConn, err := net.Dial("tcp", p.endpoint)
	if err != err {
		log.Println("Couldn't connect to endpoint:", err)
		return
	}
	defer nodeConn.Close()

	io.Copy(nodeConn, clientConn)
	io.Copy(clientConn, nodeConn)
}

func (p *Proxy) Close() {
	if p.ln != nil {
		p.ln.Close()
	}
}
