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
	CONN_TYPE      string = "tcp"
	SERVER_PORT    string = "20454"
	SERVER_VERSION string = "1.0.0"
	BUFFER_SIZE    int    = 1024

	CONN_REQUSET     string = "0"
	CONN_REJECT      string = "1"
	CONN_KILL        string = "2"
	CLIENT_BROADCAST string = "3"
	SERVER_BROADCAST string = "4"
	DIRECT_MESSAGE   string = "5"
	GET_VERSION      string = "6"
	USER_LIST        string = "7"
	GET_RTT          string = "8"

	BADWORD_STR string = "i hate professor"

	LISTENER_OPEN_ERR       string = "cannot open server"
	CONN_OPEN_ERR           string = "connection error"
	REJECT_MSG_ROMM_FULL    string = "chatting room full. cannot connect"
	REJECT_MSG_NICKNAME_DUP string = "that nickname is already used by another user. cannot connect"
	SERVER_DOWN             string = "[server has been terminated]"
	BADWORD_KILL            string = "[you used bad word]"
	EXIT_MSG                string = "gg~"
	INVALID_RECEIVER        string = "invalid direct message receiver: "
	INTERPRET_FAIL          string = "invalid message format"
)

var (
	// array can't be defined in const
	WELCOME_MSG []string = []string{
		"[welcome ",
		" to CAU network class chat room at ",
		".]\n[There are ",
		" users connected]",
	}
	CONN_SERVER_MSG []string = []string{
		"[",
		" joined from ",
		". There are ",
		" users connected]",
	}
	DISCONN_MSG []string = []string{
		"[",
		" left. There are ",
		" users now]",
	}
	FORCE_KILL_MSG []string = []string{
		"[",
		" is disconnected. There are ",
		" users in the chat room.]",
	}

	listener net.Listener

	totalClientCount   int32                  = 0                            // shared variable, thus should be thread-safe
	nicknameToSendChan map[string]chan string = make(map[string]chan string) // for broadcast, \dm called by other clients
	nicknameToConn     map[string]net.Conn    = make(map[string]net.Conn)    // for \list called by other clients and server force stop

	err error
)

func main() {
	initCtrlCHandler()

	listener, err = net.Listen(CONN_TYPE, ":"+SERVER_PORT)
	if err != nil {
		fmt.Println(LISTENER_OPEN_ERR)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil || conn == nil {
			fmt.Println(CONN_OPEN_ERR)
		} else {
			go serverTask(conn)
		}
	}
}

func serverTask(myConn net.Conn) { // main functionality of server
	defer myConn.Close()

	buffer := make([]byte, 128)
	bufferLen, err := myConn.Read(buffer)
	if err != nil || bufferLen == 0 {
		myConn.Write([]byte(CONN_REJECT + "receive error"))
		myConn.Close()
		return
	}

	myNickname := string(buffer[1:bufferLen])
	if atomic.LoadInt32(&totalClientCount) == 8 { // reject because room is full
		myConn.Write([]byte(CONN_REJECT + REJECT_MSG_ROMM_FULL))
		myConn.Close()
		return
	} else if _, exist := nicknameToSendChan[myNickname]; exist == true { // reject because nickname is already in use
		myConn.Write([]byte(CONN_REJECT + REJECT_MSG_NICKNAME_DUP))
		myConn.Close()
		return
	} else { // accept, and get into chatting room
		tmpCnt := atomic.AddInt32(&totalClientCount, 1)
		welcomeMsg := WELCOME_MSG[0] + myNickname +
			WELCOME_MSG[1] + myConn.LocalAddr().String() +
			WELCOME_MSG[2] + fmt.Sprint(tmpCnt) + WELCOME_MSG[3]
		serverMsg := CONN_SERVER_MSG[0] + myNickname + CONN_SERVER_MSG[1] + myConn.RemoteAddr().String() +
			CONN_SERVER_MSG[2] + fmt.Sprint(tmpCnt) + CONN_SERVER_MSG[3]
		myConn.Write([]byte(CONN_REQUSET + welcomeMsg))
		fmt.Println(serverMsg)
	}

	myRecvChan := make(chan string)             // receive channel: from client to server
	mySendChan := make(chan string)             // send channel: from server to client
	nicknameToSendChan[myNickname] = mySendChan // register my nickname and channel pair
	nicknameToConn[myNickname] = myConn         // register my nickname and connection pair

	defer close(myRecvChan)
	defer close(mySendChan)

	go recvHandler(myConn, myRecvChan) // from recvHandler, this goroutine gets message from client
	go sendHandler(myConn, mySendChan) //to sendHandler, this goroutine sends message to client

	for {
		recvMsg := <-myRecvChan

		if strings.HasPrefix(recvMsg, CONN_KILL) { // connection kill by client's \exit or ctrl_c
			mySendChan <- CONN_KILL

			tmpCnt := atomic.AddInt32(&totalClientCount, -1)
			delete(nicknameToSendChan, myNickname)
			delete(nicknameToConn, myNickname)
			sendMsg := DISCONN_MSG[0] + myNickname + DISCONN_MSG[1] +
				fmt.Sprint(tmpCnt) + DISCONN_MSG[2]
			fmt.Println(sendMsg)
			for _, otherChan := range nicknameToSendChan {
				otherChan <- SERVER_BROADCAST + sendMsg
			}
			break
		} else if strings.HasPrefix(recvMsg, CLIENT_BROADCAST) { // broadcasting message from client
			sendToEverybodyMsg := recvMsg[1:]
			for otherNickname, otherChan := range nicknameToSendChan {
				if otherNickname != myNickname {
					otherChan <- CLIENT_BROADCAST + myNickname + " " + sendToEverybodyMsg
				}
			}

			if strings.Contains(strings.ToLower(sendToEverybodyMsg), BADWORD_STR) { // bad word detection
				mySendChan <- CONN_KILL + BADWORD_KILL

				tmpCnt := atomic.AddInt32(&totalClientCount, -1)
				delete(nicknameToSendChan, myNickname)
				delete(nicknameToConn, myNickname)
				sendServerMsg := FORCE_KILL_MSG[0] + myNickname + FORCE_KILL_MSG[1] +
					fmt.Sprint(tmpCnt) + FORCE_KILL_MSG[2]
				fmt.Println(sendServerMsg)
				for _, otherChan := range nicknameToSendChan {
					otherChan <- SERVER_BROADCAST + sendServerMsg
				}
				break
			}
		} else if strings.HasPrefix(recvMsg, DIRECT_MESSAGE) { // dm from client to another client
			idx := 1
			for ; idx < len(recvMsg); idx++ {
				if recvMsg[idx] == ' ' {
					break
				}
			}

			receiver, sendMsg := recvMsg[1:idx], recvMsg[idx+1:]
			if _, exist := nicknameToSendChan[receiver]; exist == true {
				nicknameToSendChan[receiver] <- DIRECT_MESSAGE + myNickname + " " + sendMsg
			} else {
				fmt.Println(INVALID_RECEIVER + receiver)
			}

			if strings.Contains(strings.ToLower(sendMsg), BADWORD_STR) { // bad word detection
				mySendChan <- CONN_KILL + BADWORD_KILL

				tmpCnt := atomic.AddInt32(&totalClientCount, -1)
				delete(nicknameToSendChan, myNickname)
				delete(nicknameToConn, myNickname)
				sendMsg := FORCE_KILL_MSG[0] + myNickname + FORCE_KILL_MSG[1] +
					fmt.Sprint(tmpCnt) + FORCE_KILL_MSG[2]
				fmt.Println(sendMsg)
				for _, otherChan := range nicknameToSendChan {
					otherChan <- SERVER_BROADCAST + sendMsg
				}
				break
			}
		} else if strings.HasPrefix(recvMsg, GET_VERSION) { // \ver from client
			mySendChan <- GET_VERSION + SERVER_VERSION
		} else if strings.HasPrefix(recvMsg, USER_LIST) { // \list from client
			sendMsg := USER_LIST
			for name, otherConn := range nicknameToConn {
				sendMsg += name + ": " + otherConn.RemoteAddr().String() + "\n"
			}
			mySendChan <- sendMsg
		} else if strings.HasPrefix(recvMsg, GET_RTT) { // \rtt from client
			mySendChan <- GET_RTT
		} else {
			fmt.Println(INTERPRET_FAIL)
		}
	}
}

func recvHandler(conn net.Conn, ch chan<- string) {
	buffer := make([]byte, BUFFER_SIZE)
	var err error

	for {
		bufferLen := 0
		for bufferLen == 0 {
			bufferLen, err = conn.Read(buffer)
			if err != nil {
				return
			}
		}

		msg := string(buffer[0:bufferLen])
		ch <- msg
		time.Sleep(time.Millisecond * 10) // slow down, 'cause message are mixed up

		if strings.HasPrefix(msg, CONN_KILL) {
			break
		}
	}
}

func sendHandler(conn net.Conn, ch <-chan string) {
	for {
		msg := <-ch // receive message from server thread
		conn.Write([]byte(msg))
		time.Sleep(time.Millisecond * 10) // slow down, 'cause message are mixed up

		if strings.HasPrefix(msg, CONN_KILL) {
			break
		}
	}
}

func initCtrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		for _, conn := range nicknameToConn { // kill all client when server is died
			conn.Write([]byte(CONN_KILL + SERVER_DOWN))
			time.Sleep(time.Millisecond * 10) // slow down, 'cause message are mixed up
		}
		for _, conn := range nicknameToConn {
			conn.Close()
		}
		fmt.Println(EXIT_MSG)
		os.Exit(0)
	}()
}
