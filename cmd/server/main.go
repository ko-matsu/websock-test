// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// see: https://github.com/gorilla/websocket/blob/master/examples/echo/server.go

package main

import (
	"context"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/echo")
}

func main() {
	parentCtx := context.Background()
	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer cancel()

	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)
	srv := &http.Server{Addr: *addr}

	done := make(chan error)
	go func() {
		defer close(done)
		err := srv.ListenAndServe()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
		log.Printf("cancel context: %s\n", ctx.Err().Error())
		shutdownCh := make(chan struct{})
		go func() {
			defer close(shutdownCh)
			shutdownCtx, timeoutFn := context.WithTimeout(parentCtx, time.Second*5)
			defer timeoutFn()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Fatal(err)
			}
		}()
		<-shutdownCh
		log.Println("shutdown done.")
	}
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output" style="max-height: 70vh;overflow-y: scroll;"></div>
</td></tr></table>
</body>
</html>
`))
