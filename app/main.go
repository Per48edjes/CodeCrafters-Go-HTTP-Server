package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		os.Exit(1)
	}

	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	path := ""
	if len(parts) >= 2 {
		path = parts[1]
	}

	statusLine := "HTTP/1.1 404 Not Found\r\n\r\n"
	if path == "/" {
		statusLine = "HTTP/1.1 200 OK\r\n\r\n"
	}

	_, err = conn.Write([]byte(statusLine))
	if err != nil {
		fmt.Println("Error writing response: ", err.Error())
		os.Exit(1)
	}
}
