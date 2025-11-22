package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// No flags needed for now, connection is manual
	// host := flag.String("host", "localhost:8080", "server host:port")
	// flag.Parse()

	// Setup logging
	// f, err := tea.LogToFile("client.log", "debug")
	// if err != nil {
	// 	fmt.Println("fatal:", err)
	// 	os.Exit(1)
	// }
	// defer f.Close()

	net := NewNetwork()
	// defer net.Close() // Close when quitting

	p := tea.NewProgram(initialModel(net), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
