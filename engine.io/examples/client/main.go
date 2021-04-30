package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	eio "github.com/tomruk/socket.io-go/engine.io"
	"github.com/tomruk/socket.io-go/engine.io/parser"
)

var exitChan = make(chan struct{})

func onPacket(packet *parser.Packet) {
	if packet.Type == parser.PacketTypeMessage {
		fmt.Printf("Message received: %s\n", packet.Data)
	}
}

func onError(err error) {
	fmt.Printf("Socket error: %v\n", err)
}

func onClose(reason string, err error) {
	if err == nil {
		fmt.Printf("Socket closed: %s\n", reason)
	} else {
		fmt.Printf("Socket closed: %s | Error: %v\n", reason, err)
	}

	close(exitChan)
}

func printUpgrade(transportName string) {
	fmt.Printf("Socket is upgraded to %s\n", transportName)
}

func main() {
	config := &eio.ClientConfig{
		Transports:  []string{"polling", "websocket"},
		UpgradeDone: printUpgrade,
	}

	callbacks := &eio.Callbacks{
		OnPacket: onPacket,
		OnError:  onError,
		OnClose:  onClose,
	}

	socket, err := eio.Dial("http://127.0.0.1:3000/engine.io", callbacks, config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Connected with Session ID: %s\n", socket.ID())

	go userInput(socket)

	<-exitChan
}

func userInput(socket eio.Socket) {
	defer socket.Close()

	fmt.Printf("Type your message and press enter to send it.\n\n")

	for {
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("User input error: %v\n", err)
			break
		}
		text = strings.TrimRight(text, "\r\n")

		if text == "exit" {
			break
		}

		packet, err := parser.NewPacket(parser.PacketTypeMessage, false, []byte(text))
		if err != nil {
			fmt.Printf("Packet creation error (this shouldn't have happened): %v\n", err)
			break
		}

		socket.Send(packet)
	}
}
