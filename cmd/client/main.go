// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// see: https://github.com/gorilla/websocket/blob/master/examples/echo/client.go

package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func execWebSock(ctx context.Context, addr string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/echo"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return err
	}
	defer c.Close()

	errCh := make(chan error)

	go func() {
		defer close(errCh)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				errCh <- err
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			return err
		case t := <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				log.Println("write:", err)
				return err
			}
		case <-ctx.Done():
			log.Println("interrupt")
			shutdownCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second)
			defer timeoutFn()

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return err
			}
			select {
			case <-errCh:
			case <-shutdownCtx.Done():
			}
			return nil
		}
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	parentCtx := context.Background()
	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer cancel()

	for {
		err := execWebSock(ctx, *addr)
		if err != nil {
			log.Printf("execWebSock: %+v\n", err)
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		time.Sleep(time.Second * 5)
		log.Println("connect retry.")
	}
}
