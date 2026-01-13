package main

import (
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()

	var mu sync.RWMutex
	kv := make(map[any]any)

	n.Handle("txn", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		txn := body["txn"].([]any)

		mu.Lock()
		for _, op := range txn {
			list := op.([]any)
			opType := list[0].(string)
			key := list[1]

			switch opType {
			case "r":
				list[2] = kv[key]
			case "w":
				kv[key] = list[2]
			}
		}
		mu.Unlock()

		return n.Reply(msg, map[string]any{
			"type": "txn_ok",
			"txn":  txn,
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
