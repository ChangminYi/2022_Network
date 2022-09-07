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
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	serverName, serverPort string = "nsl2.cau.ac.kr", "20454"
	BUFFER_SIZE            int    = 1024
	ERR_SEND               int    = 1
	ERR_REC                int    = 2
)

var (
	buffer               []byte = make([]byte, BUFFER_SIZE)
	pconn                net.PacketConn
	server_addr          *net.UDPAddr
	scanner              bufio.Scanner = *bufio.NewScanner(os.Stdin)
	tmp_cnt              int
	usr_opt, str_to_send string
	start_t, end_t       float64
	err                  error
)

func main() {
	// initializing client's udp, and gets server's IP and port #.
	pconn, err = net.ListenPacket("udp", ":")
	server_addr, err = net.ResolveUDPAddr("udp", serverName+":"+serverPort)
	initCtrlCHandler() // ctrl-c handler

	fmt.Printf("Client is running on port %d\n", pconn.LocalAddr().(*net.UDPAddr).Port)
	for {
		cleanBuffer()
		printCommand()

		usr_opt = getLine()
		switch usr_opt {
		case "1": // command #1: sends input string, and receives upper-cased string.
			fmt.Printf("Input lowercase sentence: ")
			str_to_send = getLine()

			start_t = float64(time.Now().UnixMicro())
			if tmp_cnt, err = pconn.WriteTo([]byte("1"+str_to_send), server_addr); err != nil {
				errorHandle(ERR_SEND)
			}
			if tmp_cnt, _, err = pconn.ReadFrom(buffer); err != nil {
				errorHandle(ERR_REC)
			}
			end_t = float64(time.Now().UnixMicro())

			fmt.Println("\nReply from server: " + string(buffer))
			printRTT()
		case "2": // command #2: requests client's IP address and port number.
			start_t = float64(time.Now().UnixMicro())
			if tmp_cnt, err = pconn.WriteTo([]byte("2"), server_addr); err != nil {
				errorHandle(ERR_SEND)
			}
			if tmp_cnt, _, err = pconn.ReadFrom(buffer); err != nil {
				errorHandle(ERR_REC)
			}
			end_t = float64(time.Now().UnixMicro())
			ipaddr, portnum := parseIPandPort()

			fmt.Println("\nReply from Server: client IP = " + ipaddr + ", port = " + portnum)
			printRTT()
		case "3": // command #3: requests the number of reqest served since server has started.
			start_t = float64(time.Now().UnixMicro())
			if tmp_cnt, err = pconn.WriteTo([]byte("3"), server_addr); err != nil {
				errorHandle(ERR_SEND)
			}
			if tmp_cnt, _, err = pconn.ReadFrom(buffer); err != nil {
				errorHandle(ERR_REC)
			}
			end_t = float64(time.Now().UnixMicro())

			fmt.Println("\nReply from Server: requests served = " + string(buffer))
			printRTT()
		case "4": // command #4: requests the running time of server program.
			start_t = float64(time.Now().UnixMicro())
			if tmp_cnt, err = pconn.WriteTo([]byte("4"), server_addr); err != nil {
				errorHandle(ERR_SEND)
			}
			if tmp_cnt, _, err = pconn.ReadFrom(buffer); err != nil {
				errorHandle(ERR_REC)
			}
			end_t = float64(time.Now().UnixMicro())

			fmt.Println("\nReply from Server: run time = " + string(buffer))
			printRTT()
		case "5": // command #5: exit program.
			cleanupAndExit()
		default: // error handling: not defined command.
			fmt.Println("\nInvalid instruction")
		}
	}
}

/**
 * prints available options
**/
func printCommand() {
	fmt.Println("<Menu>")
	fmt.Println("1) convert text to UPPER-case")
	fmt.Println("2) get my IP address and port number")
	fmt.Println("3) get server request count")
	fmt.Println("4) get server running time")
	fmt.Println("5) exit")
	fmt.Print("Input option: ")
}

/**
 * reads input from bufio.Scanner, and returns it in string.
**/
func getLine() string {
	scanner.Scan()
	return scanner.Text()
}

/**
 * buffer cleanup function. same as server.
**/
func cleanBuffer() {
	for i := 0; i < BUFFER_SIZE; i++ {
		buffer[i] = 0
	}
}

/**
 * calculates rtt. just formatting it.
**/
func printRTT() {
	fmt.Printf("RTT = %.3f ms\n\n", (end_t-start_t)/1000)
}

/**
 * parsing XXX.XXX.XXX.XXX:####
 * into two strings, IP address and port #.
**/
func parseIPandPort() (ipaddr, portnum string) {
	tmp := string(buffer)
	for i := len(tmp) - 1; i >= 0; i-- {
		if tmp[i] == ':' {
			ipaddr, portnum = tmp[:i], tmp[i+1:]
			break
		}
	}
	return
}

/**
 * ctrl-c handler. if ctrl-c interrupt program,
 * it will call cleanup function.
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
 * checking whether message has sent correctly.
 * if not, program will stop.
**/
func errorHandle(err_code int) {
	switch err_code {
	case 1:
		fmt.Println("Error occured while sending request")
	case 2:
		fmt.Println("Error occured while receiving reply")
	}
	pconn.Close()
	os.Exit(0)
}

/**
 * cleanup function. when called, function will
 * send disconnection message to server, and
 * close the connection. and exit.
**/
func cleanupAndExit() {
	pconn.Close()
	fmt.Println("\nBye bye~")
	os.Exit(0)
}
