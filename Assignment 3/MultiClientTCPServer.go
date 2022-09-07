/**
 * 20170454 YiChangmin
 * protocol messages are same with Assignment 2.
 * to deal with multi clients, server uses goroutine.
**/

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	serverPort  string = "20454"
	BUFFER_SIZE int    = 1024
)

var (
	listener    net.Listener
	start_t     time.Time
	req_serve   int32 = 0
	totalClient int32 = 0
	curClient   int32 = 0
)

func main() {
	start_t = time.Now()                            // server running time init
	listener, _ = net.Listen("tcp", ":"+serverPort) // tcp init

	ctrlCHandler()
	go printTotalClientCount()

	fmt.Println("Server is ready to receive on port", serverPort)
	for {
		conn, _ := listener.Accept() // when connection is made, call serverThread() with conn as parameter
		if conn != nil {
			fmt.Println("Connection request from", conn.RemoteAddr().String())

			/**
			 * do multi-thread stuff
			 * all the accesss to global variable use atomic function(concurrency control)
			**/
			thrNum := atomic.AddInt32(&totalClient, 1)
			fmt.Println("Client", thrNum, "connected. Number of connected clients =", atomic.AddInt32(&curClient, 1))
			go serverThread(conn, totalClient)
		}
	}
}

/**
 * multi threaded server function
 * function is same with Assignment 2, but not cleaning up buffer
 * 'cause conn.Read() returns the length of received message
**/
func serverThread(conn net.Conn, thrNum int32) {
	buffer := make([]byte, BUFFER_SIZE)
TASK:
	for {
		count, _ := conn.Read(buffer)

		switch buffer[0] {
		case '1': // command #1: get lower case string, and returns upper case string.
			fmt.Println("Command " + string(buffer[0]))
			conn.Write(bytes.ToUpper(buffer[1:count]))
		case '2': // command #2: returns client's IP address and Port #.
			fmt.Println("Command " + string(buffer[0]))
			conn.Write([]byte(conn.RemoteAddr().String()))
		case '3': // command #3: returns the number of requests served before this command.
			fmt.Println("Command " + string(buffer[0]))
			conn.Write([]byte(strconv.Itoa(int(atomic.LoadInt32(&req_serve)))))
		case '4': // command #4: returns server's running time.
			fmt.Println("Command " + string(buffer[0]))
			dura := time.Since(start_t)
			hh := dura / time.Hour
			dura %= time.Hour
			mm := dura / time.Minute
			dura %= time.Minute
			ss := dura / time.Second
			conn.Write([]byte(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)))
		case '5': // command #5: receives client's disconnection message, and reduce total client count
			fmt.Println("Client", thrNum, "disconnected. Number of connected clients =", atomic.AddInt32(&curClient, -1))
			break TASK
		default: // error handling: not defined messages
			conn.Write([]byte("Wrong command"))
		}

		atomic.AddInt32(&req_serve, 1)
	}

	conn.Close()
}

/**
 * print the number of connected clients
 * periodically, 1 minute.
**/
func printTotalClientCount() {
	for {
		time.Sleep(time.Minute /*time.Second * 60*/)
		fmt.Println("1 minute passed. Number of connected clients =", atomic.LoadInt32(&curClient))
	}
}

/**
 * receiving ctrl-c interrupt from os
**/
func ctrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cleanupAndExit()
	}()
}

/**
 * terminating function
**/
func cleanupAndExit() {
	if listener != nil {
		listener.Close()
	}
	fmt.Println("\nBye bye~")
	os.Exit(0)
}
