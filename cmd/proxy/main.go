package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

var (
	port       = flag.String("port", "8080", "Port to listen on")
	socketDir  = flag.String("socket-dir", "/var/run/kubevirt-private", "Directory containing the virt-serial0 socket")
	listenMode = flag.String("listen", "tcp", "Listen mode: tcp or unix (unix socket at /var/run/kubevirt-private/console-proxy.sock)")
	socketName = "virt-serial0"
)

func main() {
	flag.Parse()

	// Wait for the socket to appear (virt-launcher may need time to start)
	var socketPath string
	for i := 0; i < 30; i++ {
		var err error
		socketPath, err = discoverSocketPath(*socketDir)
		if err == nil {
			break
		}
		if i == 29 {
			log.Fatalf("Failed to discover socket path after 30 attempts: %v", err)
		}
		log.Printf("Waiting for socket... (%v)", err)
		time.Sleep(2 * time.Second)
	}
	log.Printf("Using serial console socket: %s", socketPath)

	http.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, socketPath)
	})

	var ln net.Listener
	var err error

	if *listenMode == "unix" {
		sockPath := filepath.Join(*socketDir, "console-proxy.sock")
		os.Remove(sockPath) // clean up stale socket
		ln, err = net.Listen("unix", sockPath)
		if err != nil {
			log.Fatalf("Failed to listen on unix socket: %v", err)
		}
		os.Chmod(sockPath, 0666)
		log.Printf("Listening on unix socket: %s", sockPath)
	} else {
		ln, err = net.Listen("tcp", ":"+*port)
		if err != nil {
			log.Fatalf("Failed to listen on port %s: %v", *port, err)
		}
		log.Printf("Listening on TCP port %s", *port)
	}

	log.Fatal(http.Serve(ln, nil))
}

func discoverSocketPath(dir string) (string, error) {
	// Check if the socket exists directly in the directory
	directPath := filepath.Join(dir, socketName)
	if _, err := os.Stat(directPath); err == nil {
		return directPath, nil
	}

	// Fall back to searching in a single subdirectory
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var subdirs []string
	for _, f := range files {
		if f.IsDir() {
			subPath := filepath.Join(dir, f.Name(), socketName)
			if _, err := os.Stat(subPath); err == nil {
				subdirs = append(subdirs, f.Name())
			}
		}
	}

	if len(subdirs) != 1 {
		return "", fmt.Errorf("expected 1 subdirectory with %s socket, found %d", socketName, len(subdirs))
	}

	return filepath.Join(dir, subdirs[0], socketName), nil
}

func handler(w http.ResponseWriter, r *http.Request, socketPath string) {
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"binary.kubevirt.io"},
		CheckOrigin:  func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	ws.SetReadDeadline(time.Now().Add(5 * time.Minute))

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Printf("Failed to dial Unix socket: %v", err)
		return
	}
	defer conn.Close()

	errChan := make(chan error, 2)

	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				errChan <- err
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				errChan <- err
				return
			}
		}
	}()

	go func() {
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if mt != websocket.BinaryMessage {
				continue
			}
			if _, err := conn.Write(data); err != nil {
				errChan <- err
				return
			}
		}
	}()

	<-errChan
	log.Println("Connection closed")
}
