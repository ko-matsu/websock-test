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

	retry "github.com/avast/retry-go/v4"
	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func execWebSock(ctx context.Context, addr string, successFn func()) error {
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
		isFirst := false
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				errCh <- err
				return
			}
			log.Printf("recv: %s", message)
			if !isFirst {
				successFn()
				isFirst = true
			}
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

	retryCtx, retryCancel := context.WithCancel(ctx)
	var maxConsecutiveErrorCount uint = 10
	var retryCount uint
	isFirst := true
	var lastErr error
	retry.Do(
		func() error {
			err := execWebSock(ctx, *addr, func() {
				retryCount = 0  // reset error count
				isFirst = false // initial connect success
			})
			lastErr = err
			switch {
			case err == nil:
			case retryCount == maxConsecutiveErrorCount:
				log.Println("retry count error.")
				return nil
			case isFirst:
				retryCancel() // initial connect failed
				isFirst = false
				fallthrough
			default:
				log.Printf("execWebSock: %+v\n", err)
			}
			return err
		},
		retry.Context(retryCtx),
		retry.LastErrorOnly(true),
		//retry.Attempts(maxConsecutiveErrorCount),
		retry.Attempts(0), // infinity
		retry.Delay(time.Millisecond*500),
		retry.MaxDelay(time.Second*5),
		//retry.OnRetry(func(n uint, err error) {
		//	log.Printf("retry, num=%d\n", n)
		//}),
		retry.DelayType(func(_ uint, err error, config *retry.Config) time.Duration {
			delayTime := retry.BackOffDelay(retryCount, err, config)
			retryCount++
			log.Printf("retry: num=%d, delay=%d\n", retryCount, delayTime/time.Second)
			return delayTime
		}),
	)
	log.Printf("err: %+v\n", lastErr)
}
