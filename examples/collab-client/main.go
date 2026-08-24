// Command collab-client is a runnable example of the threadwave Go-native sync
// client. It connects to a threadwave (or any y-websocket) server, observes a
// shared array, appends one entry, and logs every change — local and
// remote — as the document converges.
//
// Run it against the sibling server example:
//
//	go run ./examples/collab-server &
//	go run ./examples/collab-client -url ws://localhost:8080/collab -doc room1 -name alice
//	go run ./examples/collab-client -url ws://localhost:8080/collab -doc room1 -name bob
//
// The two clients see each other's entries. The -url flag can also point at
// any compatible deployed y-websocket or Hocuspocus endpoint.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Arnavsharma2/threadwave"
	"github.com/Arnavsharma2/threadwave/client"
)

func main() {
	url := flag.String("url", "ws://localhost:8080", "server base URL (the doc name is appended as the path)")
	docName := flag.String("doc", "demo", "document name")
	name := flag.String("name", "", "value to append to the shared items array (default: hostname)")
	flag.Parse()

	entry := *name
	if entry == "" {
		entry, _ = os.Hostname()
	}

	c, items, err := newClient(*url, *docName)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Append our entry once the first handshake completes, so the edit
	// syncs rather than sitting purely local.
	go func() {
		waitSynced(c, 5*time.Second)
		txn := c.Doc().WriteTxn()
		items.Push(txn, entry)
		txn.Commit()
		log.Printf("appended %q", entry)
	}()

	log.Printf("collab-client on %s doc=%s as %q (ctrl-c to quit)", *url, *docName, entry)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}

// newClient builds the sync client and its observed "items" array. Split
// out from main so the smoke test can drive it. The observer fires for
// both local edits and applied remote updates.
func newClient(url, docName string) (*client.Client, *threadwave.Array, error) {
	c, err := client.New(client.Options{
		URL:      url,
		DocName:  docName,
		OnSynced: func(synced bool) { log.Printf("synced=%v", synced) },
		OnError:  func(err error) { log.Printf("conn error: %v", err) },
	})
	if err != nil {
		return nil, nil, err
	}
	items := threadwave.NewArray(c.Doc(), "items")
	items.Observe(func(e *threadwave.ArrayEvent) {
		log.Printf("items changed: %v", e.Delta)
	})
	return c, items, nil
}

// waitSynced blocks until the client reports synced or the deadline passes.
func waitSynced(c *client.Client, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if c.Synced() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
