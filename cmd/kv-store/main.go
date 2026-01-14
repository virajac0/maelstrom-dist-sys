package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()

	var mu sync.RWMutex
	kv := make(map[any]any)

	go func() {
		for {
			time.Sleep(500 * time.Millisecond)

			mu.RLock()
			snapshot := make(map[any]any)
			for k, v := range kv {
				snapshot[k] = v
			}
			mu.RUnlock()

			for _, neighbor := range n.NodeIDs() {
				if neighbor == n.ID() {
					continue
				}
				n.Send(neighbor, map[string]any{
					"type": "gossip",
					"kv":   snapshot,
				})
			}
		}
	}()

	n.Handle("gossip", func(msg maelstrom.Message) error {
		var body struct {
			KV map[string]any `json:"kv"`
		}
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		mu.Lock()
		for k, v := range body.KV {
			// Because writes are unique, we can just fill in what we are missing.
			// If we already have the key, we don't need to do anything since it won't change.
			if _, exists := kv[k]; !exists {
				kv[k] = v
			}
		}
		mu.Unlock()
		return nil
	})

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
