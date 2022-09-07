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
	pconn            net.PacketConn
	sender_addr      net.Addr
	count, req_serve int
	start_t          time.Time
	dura             time.Duration
	err              error
)

func main() {
	start_t = time.Now()                                 // runtime calculation start
	pconn, err = net.ListenPacket("udp", ":"+serverPort) //initializing server's udp
	initCtrlCHandler()                                   //ctrl-c handler init

	fmt.Println("Server is ready to receive on port " + serverPort)
	for {
		cleanBuffer() // waiting for udp packet to be sent
		if count, sender_addr, err = pconn.ReadFrom(buffer); err == nil {
			fmt.Println("UDP message from " + sender_addr.String())
		}

		// no "command #5" 'cause udp doesn't make strong connection.
		switch buffer[0] {
		case '1': // command #1: get lower case string, and returns upper case string.
			fmt.Println("Command " + string(buffer[0]))
			pconn.WriteTo(bytes.ToUpper(buffer[1:count]), sender_addr)
		case '2': // command #2: returns client's IP address and Port #.
			fmt.Println("Command " + string(buffer[0]))
			pconn.WriteTo([]byte(sender_addr.String()), sender_addr)
		case '3': // command #3: returns the number of requests served before this command.
			fmt.Println("Command " + string(buffer[0]))
			pconn.WriteTo([]byte(strconv.Itoa(req_serve)), sender_addr)
		case '4': // command #4: returns server's running time.
			fmt.Println("Command " + string(buffer[0]))
			dura = time.Since(start_t)
			hh, mm, ss := getRuntime()
			pconn.WriteTo([]byte(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)), sender_addr)
		default: // error handling: not defined messages
			pconn.WriteTo([]byte("Wrong command"), sender_addr)
		}

		req_serve++
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
	pconn.Close()
	fmt.Println("\nBye bye~")
	os.Exit(0)
}
