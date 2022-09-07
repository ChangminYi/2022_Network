/**
 * Author: 20170454 YiChangmin
 **/

/**
 * client's application message format = <command><data> (string)
 * server's application message format = <data> (string)
 * interpreting server's message done in client.
 * <command> : one ASCII character number ('0' ~ '9').
 * <data> : string
**/

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	serverPort  string = "20454"
	BUFFER_SIZE int    = 1024
)

var (
	buffer           []byte = make([]byte, BUFFER_SIZE)
	listener         net.Listener
	conn             net.Conn
	count, req_serve int
	start_t          time.Time
	dura             time.Duration
	err              error
)

func main() {
	start_t = time.Now()                              // runtime calculation start
	listener, err = net.Listen("tcp", ":"+serverPort) // tcp init
	initCtrlCHandler()                                //ctrl-c handler init

	fmt.Printf("Server is ready to receive on port %s\n", serverPort)
	for {
		conn, err = listener.Accept() // connect to client, and ready to receive/send messages.
		fmt.Printf("Connection request from %s\n", conn.RemoteAddr().String())

	TASK:
		for {
			cleanBuffer()
			count, err = conn.Read(buffer)

			switch buffer[0] {
			case '1': // command #1: get lower case string, and returns upper case string.
				fmt.Println("Command " + string(buffer[0]))
				conn.Write(bytes.ToUpper(buffer[1:count]))
			case '2': // command #2: returns client's IP address and Port #.
				fmt.Println("Command " + string(buffer[0]))
				conn.Write([]byte(conn.RemoteAddr().String()))
			case '3': // command #3: returns the number of requests served before this command.
				fmt.Println("Command " + string(buffer[0]))
				conn.Write([]byte(strconv.Itoa(req_serve)))
			case '4': // command #4: returns server's running time.
				fmt.Println("Command " + string(buffer[0]))
				dura = time.Since(start_t)
				hh, mm, ss := getRuntime()
				conn.Write([]byte(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)))
			case '5': // command #5: receives client's disconnection message, and waits for new connection.
				fmt.Println("Client has disconnected, waiting for new connection...")
				break TASK
			default: // error handling: not defined messages
				conn.Write([]byte("Wrong command"))
			}

			req_serve++
		}
		conn.Close()
	}
}

/**
 * interpreting time.Duration to Hour, Minute, and Second.
**/
func getRuntime() (hh, mm, ss time.Duration) {
	hh = dura / time.Hour
	dura %= time.Hour
	mm = dura / time.Minute
	dura %= time.Minute
	ss = dura / time.Second
	return
}

/**
 * for not allocating buffer every time,
 * program should clean buffer every time
**/
func cleanBuffer() {
	for i := 0; i < BUFFER_SIZE; i++ {
		buffer[i] = 0
	}
}

/**
 * ctrl-c handler. handler will call cleanup function
 * when ctrl-c interrupt has benn detected.
**/
func initCtrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cleanupAndExit()
	}()
}

/**
 * cleanup function. it closes tcp connection
 * and print exit message, and stop the program.
**/
func cleanupAndExit() {
	if conn != nil {
		conn.Close()
	}
	if listener != nil {
		listener.Close()
	}
	fmt.Println("\nBye bye~")
	os.Exit(0)
}
