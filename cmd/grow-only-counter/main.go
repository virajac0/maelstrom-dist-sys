package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	kv := maelstrom.NewSeqKV(n)

	n.Handle("add", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		delta := int(body["delta"].(float64))

		for {
			current, err := kv.ReadInt(context.Background(), n.ID())
			if err != nil {
				current = 0
			}

			newTotal := current + delta
			err = kv.CompareAndSwap(context.Background(), n.ID(), current, newTotal, true)

			if err == nil {
				break
			}
		}

		return n.Reply(msg, map[string]string{"type": "add_ok"})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		total := 0
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		time.Sleep(50 * time.Millisecond)
		for _, node := range n.NodeIDs() {
			value, err := kv.ReadInt(context.Background(), node)
			if err != nil {
				value = 0
			}
			total += value
		}

		return n.Reply(msg, map[string]any{"type": "read_ok", "value": total})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
