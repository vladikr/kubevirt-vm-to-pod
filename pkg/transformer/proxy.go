package transformer

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

var (
	port       = flag.String("port", "8080", "Port to listen on")
	socketDir  = flag.String("socket-dir", "/var/run/kubevirt-private", "Directory containing the virt-serial0 socket")
	socketName = "virt-serial0"
)

func main() {
	flag.Parse()

	// Discover the socket path
	socketPath, err := discoverSocketPath(*socketDir)
	if err != nil {
		log.Fatalf("Failed to discover socket path: %v", err)
	}
	log.Printf("Using serial console socket: %s", socketPath)

	http.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, socketPath)
	})

	log.Printf("Starting proxy server on port %s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

// discoverSocketPath finds the virt-serial0 socket
func discoverSocketPath(dir string) (string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var subdirs []string
	for _, f := range files {
		if f.IsDir() {
			subdirs = append(subdirs, f.Name())
		}
	}

	if len(subdirs) != 1 {
		return "", io.EOF
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
