package main

import (
	"context"
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	kv := maelstrom.NewLinKV(n)

	n.Handle("send", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		key := body["key"].(string)
		val := body["msg"]

		var assignedOffset int

		for {
			var log []any
			res, err := kv.Read(context.Background(), key)

			if err != nil {
				log = []any{}
			} else {
				log = res.([]any)
			}

			assignedOffset = len(log)
			newLog := append(log, val)

			err = kv.CompareAndSwap(context.Background(), key, log, newLog, true)
			if err == nil {
				break
			}
		}

		return n.Reply(msg, map[string]any{
			"type":   "send_ok",
			"offset": assignedOffset,
		})
	})

	n.Handle("poll", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		requestedOffsets := body["offsets"].(map[string]any)
		resultMsgs := make(map[string][][]any)

		for key, startVal := range requestedOffsets {
			startOffset := int(startVal.(float64))

			res, err := kv.Read(context.Background(), key)
			if err != nil {
				continue
			}
			log := res.([]any)

			var collected [][]any
			for i := startOffset; i < len(log); i++ {
				collected = append(collected, []any{i, log[i]})
			}

			resultMsgs[key] = collected
		}

		return n.Reply(msg, map[string]any{
			"type": "poll_ok",
			"msgs": resultMsgs,
		})
	})

	n.Handle("commit_offsets", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		offsets := body["offsets"].(map[string]any)

		for key, val := range offsets {
			offset := int(val.(float64))
			kv.Write(context.Background(), "commit_"+key, offset)
		}

		return n.Reply(msg, map[string]any{"type": "commit_offsets_ok"})
	})

	n.Handle("list_committed_offsets", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		keys := body["keys"].([]any)
		results := make(map[string]int)

		for _, k := range keys {
			key := k.(string)
			val, err := kv.ReadInt(context.Background(), "commit_"+key)
			if err == nil {
				results[key] = val
			}
		}

		return n.Reply(msg, map[string]any{
			"type":    "list_committed_offsets_ok",
			"offsets": results,
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
