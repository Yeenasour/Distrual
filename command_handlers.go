package main

import (
	"fmt"
	"io"
	"strconv"
)

type CommandHandler func(h *Hub, args []string) CommandResult

type CommandResult struct {
	Output string
	Error  error
	Done   bool
}

var handlers = map[string]CommandHandler{
	"ping":    handlePing,
	"create":  handleCreate,
	"kill":    handleKill,
	"killall": handleKillAll,
	"list":    handleList,
	"exit":    handleExit,
}

func handlePing(h *Hub, args []string) CommandResult {
	if len(args) != 2 {
		return CommandResult{Error: fmt.Errorf("Wrong number of arguments")}
	}
	from, _ := strconv.Atoi(args[0])
	to, _ := strconv.Atoi(args[1])
	if !h.IsValidNodeID(from) || !h.IsValidNodeID(to) {
		return CommandResult{Error: fmt.Errorf("Argument \"from\" or \"to\" outside valid range")}
	}

	stdin := h.nodes[from].stdin
	addr := h.proxies[to].ln.Addr().String()

	err := writeCommand(stdin, "ping %s\n", addr)
	if err != nil {
		return CommandResult{Error: err}
	}
	return CommandResult{}
}

func handleCreate(h *Hub, args []string) CommandResult {
	if len(args) < 1 {
		return CommandResult{Error: fmt.Errorf("Must provide at least a binary to run")}
	}
	err := h.StartNode(args[0], args[1:]...)
	if err != nil {
		return CommandResult{Error: fmt.Errorf("Failed to start process, %s", err)}
	}
	return CommandResult{}
}

func handleKill(h *Hub, args []string) CommandResult {
	if len(args) != 1 {
		return CommandResult{Error: fmt.Errorf("Most provide a node ID")}
	}
	nodeID, _ := strconv.Atoi(args[0])
	h.RemoveNode(nodeID)
	return CommandResult{}
}

func handleKillAll(h *Hub, args []string) CommandResult {
	h.ReapNodes()
	return CommandResult{}
}

func handleList(h *Hub, args []string) CommandResult {
	str := ""
	for i := range h.nodes {
		p := h.proxies[i]
		str += fmt.Sprintf("(%d - {n - %s. p - %s})\n", i, p.endpoint, p.ln.Addr().String())
	}
	return CommandResult{Output: str}
}

func handleExit(h *Hub, args []string) CommandResult {
	h.ReapNodes()
	return CommandResult{Done: true}
}

func writeCommand(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}
