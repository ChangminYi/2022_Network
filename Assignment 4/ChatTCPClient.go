package main

/**
 * 20170454 Yi Changmin
 */

/**
MESSAGE FORMAT
"0": connection request, connection accept
	client: "0"[nickname]
	server: "0"[welcomeMsg]
"1": connection request reject
	server: "1"[rejectReason]
"2": connection kill
	client: "2"
	server: "2"[killReason]
"3": broadcast message by client
	client: "3"[senderNickname]" "[msg]
"4": broadcast message by server
	server: "4"[msg]
"5": direct message
	sender: "5"[receiverNickname]" "[msg]
	server changes [receiverNickname] to [senderNickname]
	receiver: "5"[senderNickname]" "[msg]
"6": version of server
	client: "6"
	server: "6"[serverVersion]
"7": list of all users
	client: "7"
	server: "7"[listMsg]
"8": rtt
	client: "8"
	server: "8"
*/

import (
	"bufio"
	"container/list"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	CONN_TYPE   string = "tcp"
	SERVER_NAME string = "nsl2.cau.ac.kr"
	SERVER_PORT string = "20454"
	BUFFER_SIZE int    = 1024

	CONN_REQUSET     string = "0"
	CONN_REJECT      string = "1"
	CONN_KILL        string = "2"
	CLIENT_BROADCAST string = "3"
	SERVER_BROADCAST string = "4"
	DIRECT_MESSAGE   string = "5"
	GET_VERSION      string = "6"
	USER_LIST        string = "7"
	GET_RTT          string = "8"

	NO_SERVER_FOUND  string = "cannot find server"
	INVALID_ARG      string = "invalid argument"
	INVALID_COMMAND  string = "invalid command"
	INVALID_MSG_RECV string = "invalid message from server"
	SERVER_LOST      string = "server is not good"
	EXIT_MSG         string = "gg~"
)

var (
	scanner bufio.Scanner = *bufio.NewScanner(os.Stdin)

	myNickname    string
	conn          net.Conn
	startTimeList list.List = list.List{}

	sendChan, receiveChan chan string

	terminateChan chan bool
	terminateFlag int32 = 0

	err error
)

func main() {
	initCtrlCHandler()

	if len(os.Args) != 2 { // argument format checking
		fmt.Println(INVALID_ARG)
		return
	} else {
		myNickname = os.Args[1]
	}

	conn, err = net.Dial(CONN_TYPE, SERVER_NAME+":"+SERVER_PORT) // connection start
	if err != nil {
		fmt.Println(NO_SERVER_FOUND)
		return
	}
	defer conn.Close()

	// trying to get into chatting room
	buffer := make([]byte, 128)
	conn.Write([]byte(CONN_REQUSET + myNickname))
	bufferLen, err := conn.Read(buffer)
	if err != nil {
		fmt.Println(SERVER_LOST)
		return
	}

	msg := string(buffer[0:bufferLen])
	fmt.Println(msg[1:])
	if strings.HasPrefix(msg, CONN_REJECT) {
		return
	}

	terminateChan = make(chan bool) // for ctrl_c, and \exit
	sendChan = make(chan string)    // send message to server by this channel
	receiveChan = make(chan string) // receive message from server by this channel
	defer close(sendChan)
	defer close(receiveChan)
	defer close(terminateChan)

	go receiveThread(conn, receiveChan) // sending goroutine
	go sendThread(conn, sendChan)       // receiving goroutine

	<-terminateChan
}

func sendThread(myConn net.Conn, ch chan string) {
	go sendHandler(myConn, ch)

	for {
		scanner.Scan()
		input := scanner.Text()
		if len(input) == 0 {
			continue
		}

		if strings.HasPrefix(input, "\\") { // check whether input is command
			if input == "\\list" {
				ch <- USER_LIST
			} else if input == "\\exit" {
				ch <- CONN_KILL
				break
			} else if input == "\\ver" {
				ch <- GET_VERSION
			} else if input == "\\rtt" {
				ch <- GET_RTT
				startTimeList.PushBack(float64(time.Now().UnixMicro()))
			} else if strings.HasPrefix(input, "\\dm ") {
				msg := input[4:]
				if strings.Contains(msg, " ") && !strings.Contains(msg, "\\") {
					ch <- DIRECT_MESSAGE + msg
				} else {
					fmt.Println(INVALID_COMMAND)
				}
			} else {
				fmt.Println(INVALID_COMMAND)
			}
		} else { // broadcasting message
			ch <- CLIENT_BROADCAST + input
		}
	}
}

func receiveThread(myConn net.Conn, ch chan string) {
	for {
		go receiveHandler(myConn, ch)
		msg := <-ch

		if strings.HasPrefix(msg, CONN_KILL) { // client kill acception, force kill by server
			killMsg := msg[1:]
			if len(killMsg) > 0 {
				fmt.Println(msg[1:])
			}
			break
		} else if strings.HasPrefix(msg, CLIENT_BROADCAST) { // receiving broadcast message
			idx := 0
			for ; idx < len(msg); idx++ {
				if msg[idx] == ' ' {
					break
				}
			}

			fmt.Println(msg[1:idx] + "> " + msg[idx+1:])
		} else if strings.HasPrefix(msg, SERVER_BROADCAST) { // receiving server message
			fmt.Println(msg[1:])
		} else if strings.HasPrefix(msg, DIRECT_MESSAGE) { // receiving dm from another client
			idx := 0
			for ; idx < len(msg); idx++ {
				if msg[idx] == ' ' {
					break
				}
			}

			fmt.Println("from: " + msg[1:idx] + "> " + msg[idx+1:])
		} else if strings.HasPrefix(msg, GET_VERSION) { // receiving ver
			fmt.Println("Server version: " + msg[1:])
		} else if strings.HasPrefix(msg, USER_LIST) { // receiving list
			fmt.Println("User List:")
			fmt.Print(msg[1:])
		} else if strings.HasPrefix(msg, GET_RTT) { // calculating rtt
			endTime := float64(time.Now().UnixMicro())
			startTime := startTimeList.Front().Value.(float64)
			startTimeList.Remove(startTimeList.Front())
			fmt.Printf("RTT: %.3fms\n", (endTime-startTime)/1000)
		} else {
			if len(msg) > 0 {
				fmt.Println(INVALID_MSG_RECV + ": " + msg)
			}
		}
	}

	cleanupAndExit()
}

func sendHandler(myConn net.Conn, ch <-chan string) {
	for sendChan != nil {
		msg := <-ch // receive message to send from send thread
		_, err := myConn.Write([]byte(msg))
		time.Sleep(time.Millisecond * 10) // slow down, 'cause message are mixed up
		if err != nil {
			fmt.Println(SERVER_LOST)
			cleanupAndExit()
		} else if strings.HasPrefix(msg, CONN_KILL) { // client will be terminated
			cleanupAndExit()
		}
	}
}

func receiveHandler(myConn net.Conn, ch chan<- string) {
	buffer := make([]byte, BUFFER_SIZE)
	bufferLen := 0
	for bufferLen == 0 {
		bufferLen, err = myConn.Read(buffer)
		if err != nil {
			return
		}
	}

	msg := string(buffer[0:bufferLen])
	ch <- msg
	time.Sleep(time.Millisecond * 10) // slow down, 'cause message are mixed up
}

func initCtrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		sendChan <- CONN_KILL
	}()
}

func cleanupAndExit() {
	if atomic.AddInt32(&terminateFlag, 1) == 1 { // concurrency control: do just one time
		fmt.Println(EXIT_MSG)
		terminateChan <- true // terminate client's main function
	} else {
		return
	}
}
