package event

import (
	"encoding/json"
	"os"
)

type EventType int

const (
	Init EventType = iota
	Snapshot
	Command
)

type Event struct {
	Type    EventType
	NodeID  int
	Payload interface{}
}

func WriteEvent(e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, byte('\n'))
	os.Stdout.Write(data)
	return nil
}

func DecodeEvent(data []byte) (*Event, error) {
	msg := Event{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
