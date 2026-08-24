// Command threadwave-server is the legacy name of the stand-alone WebSocket
// sync server for threadwave documents.
//
// Deprecated: use cmd/threadserve instead — same server, plus
// auto-versioning flags; new server features land there. threadwave-server
// remains a working alias for existing deployments and will be
// removed in a future major release.
//
// Usage:
//
//	threadwave-server [-addr :8080] [-store path/to/threadwave.db]
//
// Without -store the server runs purely in-memory; documents are
// lost when their last connection disconnects. With -store the
// server persists every applied update to a SQLite database and
// loads the document history on first connect of a fresh server
// process.
//
// Mount point: documents are addressed by the URL path. A client
// connecting to ws://host:8080/my-doc operates on docName
// "my-doc". The leading slash is stripped; query strings are
// ignored (matching y-websocket's convention).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Arnavsharma2/threadwave/persist"
	"github.com/Arnavsharma2/threadwave/persist/sqlite"
	"github.com/Arnavsharma2/threadwave/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	storePath := flag.String("store", "", "SQLite database path for persistence (empty = in-memory)")
	flag.Parse()

	var store persist.Store
	if *storePath != "" {
		s, err := sqlite.Open(*storePath)
		if err != nil {
			log.Fatalf("threadwave-server: open store %q: %v", *storePath, err)
		}
		defer s.Close()
		store = s
		log.Printf("threadwave-server: persistence enabled (sqlite at %s)", *storePath)
	} else {
		log.Printf("threadwave-server: in-memory only (pass -store to persist)")
	}

	srv := server.New(server.Options{
		Store:          store,
		OriginPatterns: []string{"*"}, // dev-friendly; tighten in prod
	})

	httpSrv := &http.Server{
		Addr:    *addr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	idleConnsClosed := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Printf("threadwave-server: shutting down")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("threadwave-server: HTTP shutdown: %v", err)
		}
		if err := srv.Close(ctx); err != nil {
			log.Printf("threadwave-server: store flush: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("threadwave-server: listening on %s", *addr)
	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("threadwave-server: %v", err)
	}
	<-idleConnsClosed
	log.Printf("threadwave-server: stopped")
	fmt.Fprintln(os.Stderr, "")
}
