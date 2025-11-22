package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var addr = flag.String("addr", ":8080", "http service address")

func setupLogging() (*os.File, error) {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile("logs/server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return logFile, nil
}

func compressLog() {
	source := "logs/server.log"
	timestamp := time.Now().Format("20060102-150405")
	target := fmt.Sprintf("logs/logs-%s.tar.gz", timestamp)

	file, err := os.Open(source)
	if err != nil {
		log.Printf("Failed to open log for compression: %v", err)
		return
	}
	defer file.Close()

	outFile, err := os.Create(target)
	if err != nil {
		log.Printf("Failed to create compressed log file: %v", err)
		return
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	info, err := file.Stat()
	if err != nil {
		log.Printf("Failed to stat log file: %v", err)
		return
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		log.Printf("Failed to create tar header: %v", err)
		return
	}
	header.Name = "server.log"

	if err := tw.WriteHeader(header); err != nil {
		log.Printf("Failed to write tar header: %v", err)
		return
	}

	if _, err := io.Copy(tw, file); err != nil {
		log.Printf("Failed to compress log: %v", err)
		return
	}

	log.Printf("Log compressed to %s", target)
}

func main() {
	logFile, err := setupLogging()
	if err != nil {
		fmt.Printf("Failed to setup logging: %v\n", err)
		return
	}
	defer logFile.Close()

	configFile := flag.String("config", "serverconfig.json", "Path to configuration file")
	flag.Parse()

	config := NewConfig(*configFile)
	if err := config.Load(); err != nil {
		log.Printf("Error loading config: %v", err)
	}

	store := NewStore("users.json", "messages.json")
	if err := store.Load(); err != nil {
		log.Printf("Error loading store: %v", err)
	}

	hub := NewHub(store, config)
	go hub.Run()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>CMPPChat Server</title>
    <style>
        body { font-family: sans-serif; text-align: center; padding-top: 50px; }
        code { background: #f4f4f4; padding: 5px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>Welcome to CMPPChat Server</h1>
    <p>This is the server endpoint.</p>
    <p>Please use the TUI client to connect.</p>
    <p>Run: <code>./client -host %s</code></p>
</body>
</html>
`, config.Host)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	http.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow CORS

		messages := store.GetMessages()
		// Optional: limit query param ?limit=100
		// For now return all

		if err := json.NewEncoder(w).Encode(messages); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	serverAddr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	server := &http.Server{Addr: serverAddr}

	serverStopped := make(chan struct{}) // Channel to signal server goroutine completion

	go func() {
		log.Printf("Server started on %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
		close(serverStopped) // Signal that the server goroutine has finished
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		fmt.Println("\nShutting down server...")
		compressLog()
		os.Remove("logs/server.log")
		os.Exit(0)
	}()

	// Server Console Loop
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Server console ready. Type 'help' for commands.")
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "help":
			fmt.Println("Available commands: ban <ipid>, kick <ipid>, unban <ipid>, broadcast <msg>, stop")
		case "stop":
			fmt.Println("Stopping server...")
			return
		case "kick":
			if len(args) != 1 {
				fmt.Println("Usage: kick <ipid>")
				continue
			}
			if hub.KickUser(args[0]) {
				fmt.Println("User kicked.")
			} else {
				fmt.Println("User not found.")
			}
		case "ban":
			if len(args) != 1 {
				fmt.Println("Usage: ban <ipid>")
				continue
			}
			if err := config.Ban(args[0]); err != nil {
				fmt.Println("Error banning:", err)
			} else {
				fmt.Println("User banned.")
				hub.KickUser(args[0])
			}
		case "unban":
			if len(args) != 1 {
				fmt.Println("Usage: unban <ipid>")
				continue
			}
			if err := config.Unban(args[0]); err != nil {
				fmt.Println("Error unbanning:", err)
			} else {
				fmt.Println("User unbanned.")
			}
		case "broadcast":
			if len(args) < 1 {
				fmt.Println("Usage: broadcast <message>")
				continue
			}
			msg := strings.Join(args, " ")
			hub.BroadcastSystemMessage("[Admin] " + msg)
			fmt.Println("Broadcast sent.")
		default:
			fmt.Println("Unknown command.")
		}
	}
}
