# Maelstrom Distributed Systems Challenges

This repository contains my solutions to the [Gossip Glomers](https://fly.io/dist-sys/) distributed systems challenges using **Go** and the **Maelstrom** testing runtime.

## Project Structure

The project uses a standard Go `cmd` layout to manage multiple binaries within a single module.

* `cmd/echo/`: Source code for Challenge #1.
* `cmd/unique-ids/`: Source code for Challenge #2.
* `go.mod`: Project dependencies and module definition (`maelstrom-challenges`).

## Completed Challenges

### 1. Echo
**Goal:** Implement a node that responds to "echo" messages with an "echo_ok" message.
* **Key Learnings:** Standard input/output protocol for Maelstrom and basic JSON message handling.
* **Build:** `go build -o maelstrom-echo ./cmd/echo`.

### 2. Unique ID Generation
**Goal:** Implement a globally unique ID generator that is **totally available**, even during network partitions.
* **Key Learnings:**
    * Implementing thread-safe counters in Go using the `sync/atomic` package to ensure concurrency safety.
    * The approach involves using a combination of the node ID and an atomic counter to ensure uniqueness without cross-node communication. This enables us to prioritize availability over consistency in the event of network partitions. 
* **Build:** `go build -o maelstrom-unique-ids ./cmd/unique-ids`.
  
## Testing

To run the challenges, ensure you have the `maelstrom` binary installed and run the following commands from the root directory:

```bash
# Challenge 1: Echo
./maelstrom test -w echo --bin ./maelstrom-echo --node-count 1 --time-limit 10

# Challenge 2: Unique IDs
./maelstrom test -w unique-ids --bin ./maelstrom-unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition
