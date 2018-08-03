package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"io"
	"net/http"

	"time"

	"os/exec"

	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
	blackfriday "gopkg.in/russross/blackfriday.v2"
)

var logFlag = false

func markdownFileToHTML(filename string) []byte {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return blackfriday.Run(bytes, blackfriday.WithExtensions(blackfriday.Tables|blackfriday.FencedCode))
}

func logPrintln(v ...interface{}) {
	if logFlag {
		log.Println(v)
	}
}

var template = `
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8"/>
    <title>Document</title>
  </head>
  <body>
    <div id="content"></div>
    <script>
     window.onload = function () {
         if (!window["WebSocket"]) {
             alert("エラー : WebSocketに対応していないブラウザです。")
         } else {
             socket = new WebSocket("ws://localhost:1129/ws");
             socket.onclose = function() {
                 alert(" 接続が終了しました。");
             }
             socket.onmessage = function(e) {
                 var content = document.getElementById('content');
                 content.innerHTML = e.data;
             }
         }
     };
    </script>
  </body>
</html>
`

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	var err error

	filename := os.Args[1]

	result := markdownFileToHTML(filename)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err = io.WriteString(w, template)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	err = watcher.Add(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			return
		}

		result = markdownFileToHTML(filename)
		ws.WriteMessage(websocket.TextMessage, result)

		go func() {
			for {
				select {
				case event := <-watcher.Events:
					result = markdownFileToHTML(filename)
					ws.WriteMessage(websocket.TextMessage, result)
					logPrintln("event:", event)
					if event.Op&fsnotify.Write == fsnotify.Write {
						logPrintln("modified file:", event.Name)
					}
				case err := <-watcher.Errors:
					logPrintln("WatchError:", err)
				}
			}
		}()
		for {
			time.Sleep(1 * time.Second)
		}
	})

	go func() {
		if err = http.ListenAndServe(":1129", nil); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}()

	go func() {
		if err = browser.OpenURL("http://localhost:1129"); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		fmt.Fprintf(os.Stderr, "Set $EDITOR\n")
		os.Exit(1)
	}

	splitted := strings.Split(editor, " ")
	log.Printf("splitted: %#v\n", splitted)
	cname := splitted[0]
	args := splitted[1:]
	args = append(args, filename)

	cmd := exec.Command(cname, args[:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "editor error: %v\n", err)
		os.Exit(1)
	}
}
