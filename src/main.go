package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"./backend/message"
)

func main() {
	fmt.Println("Chat Application")
	fmt.Println("Type 'exit' to quit.")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if strings.ToLower(input) == "exit" {
			fmt.Println("Exiting chat.")
			break
		}

		userMessage := message.NewUserMessage(input)
		fmt.Printf("User Message: %+v\n", userMessage)

		// ここにアシスタントの応答ロジックを追加します
		assistantResponse := "これはアシスタントの応答です: " + userMessage.Content
		assistantMessage := message.NewAssistantMessage(assistantResponse)
		fmt.Printf("Assistant: %s\n", assistantMessage.Content)
	}
}