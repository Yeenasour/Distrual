package msg

import (
	"encoding/json"
	"os"
)

type MsgType int

const (
	Event MsgType = iota
	Init
	Snapshot
	Command
)

type Message struct {
	Type    MsgType
	Payload interface{}
}

func WriteMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, byte('\n'))
	os.Stdout.Write(data)
	return nil
}

func DecodeMessage(data []byte) (*Message, error) {
	msg := Message{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
