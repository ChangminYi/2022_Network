/**
* 20170454 Yi Changmin
**/

/**
* TCP HEADER
* "0": TCP_CONN_REQUEST
	client: "0"[nickname]" "[UDP port number]
	server: "0"[welcome message]
* "1": TCP_CONN_REJECT
	server: "1"[reject reason]
* "2": TCP_OPPONENT_DATA
	server: "2"[opponent udp address]" "[opponent nickname]" "[my playing symbol]
* "3": TCP_CONN_QUIT
	client: "3"
**/

package main

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
	CONN_TYPE   string = "tcp"
	SERVER_PORT string = "50454"
	BUFFER_SIZE int    = 1024

	TCP_CONN_REQUEST  string = "0"
	TCP_CONN_REJECT   string = "1"
	TCP_OPPONENT_DATA string = "2"
	TCP_CONN_QUIT     string = "3"
)

var (
	PLAYER_SYMBOL [2]string = [2]string{"O", "@"}

	listener net.Listener
	buffer   []byte = make([]byte, BUFFER_SIZE)
	bufLen   int

	conn     [2]net.Conn  = [2]net.Conn{nil, nil}
	nickname [2]string    = [2]string{}
	ipAddr   [2]string    = [2]string{}
	udpPort  [2]string    = [2]string{}
	quitChan [2]chan bool = [2]chan bool{}
	connCnt  int32        = 0
)

func main() {
	initCtrlCHandler()

	listener, _ = net.Listen(CONN_TYPE, ":"+SERVER_PORT)

	for idx := 0; true; idx = (idx + 1) % 2 {
		if atomic.LoadInt32(&connCnt) == 2 { // two users connected, pair each other and disconnect
			for _, ch := range quitChan {
				ch <- true
			}

			// first join, first play
			conn[idx].Write([]byte(TCP_OPPONENT_DATA + ipAddr[(idx+1)%2] + ":" + udpPort[(idx+1)%2] + " " + nickname[(idx+1)%2] + " " + PLAYER_SYMBOL[0]))
			conn[(idx+1)%2].Write([]byte(TCP_OPPONENT_DATA + ipAddr[idx] + ":" + udpPort[idx] + " " + nickname[idx] + " " + PLAYER_SYMBOL[1]))

			fmt.Println(nickname[idx] + " and " + nickname[(idx+1)%2] + " disconnected.")
			for i := 0; i < 2; i++ {
				conn[i].Close()
				ipAddr[i], udpPort[i], nickname[i], conn[i] = "", "", "", nil
			}
			atomic.AddInt32(&connCnt, -2)
		} else if conn[idx] == nil { // empty slot of user, acccept new user
			tmpConn, _ := listener.Accept()
			bufLen, _ := tmpConn.Read(buffer)
			if bufLen == 0 {
				break
			}
			splittedMsg := strings.Split(string(buffer[1:bufLen]), " ")
			tmpNickname, tmpUDPPort := splittedMsg[0], splittedMsg[1]
			if tmpNickname == nickname[(idx+1)%2] {
				tmpConn.Write([]byte(TCP_CONN_REJECT + "that nickname is already in use"))
				tmpConn.Close()
			} else {
				conn[idx], nickname[idx], ipAddr[idx], udpPort[idx] = tmpConn, tmpNickname, strings.Split(tmpConn.RemoteAddr().String(), ":")[0], tmpUDPPort
				atomic.AddInt32(&connCnt, 1)
				quitChan[idx] = make(chan bool)
				go quitHandler(idx, quitChan[idx])

				fmt.Println(nickname[idx] + " joined from " + conn[idx].RemoteAddr().String() + ". UDP port " + udpPort[idx] + ".")
				if curCnt := atomic.LoadInt32(&connCnt); curCnt == 1 {
					tmpConn.Write([]byte(TCP_CONN_REQUEST + "Welcome " + nickname[idx] + " to p2p-omok server at " + conn[idx].LocalAddr().String() + ".\nwaiting for an opponent."))
					fmt.Println("1 user connected, waiting for another")
				} else {
					tmpConn.Write([]byte(TCP_CONN_REQUEST + "Welcome " + nickname[idx] + " to p2p-omok server at " + conn[idx].LocalAddr().String() + ".\n" + nickname[(idx+1)%2] + " is waiting for you (" + ipAddr[idx] + ":" + udpPort[idx] + ")."))
					fmt.Println("2 users connected, notifying " + nickname[idx] + " and " + nickname[(idx+1)%2] + ".")
				}
			}
		} else { // meet filled slot, continue to next empty slot or pairing
			continue
		}
	}
}

/**
 * handling of waiting client disconnection.
**/
func quitHandler(idx int, quitChan chan bool) {
	defer close(quitChan)

	myBuf := make([]byte, BUFFER_SIZE)
	var myBufLen int
	var err error

	for {
		select {
		case <-quitChan:
			return
		default:
			conn[idx].SetReadDeadline(time.Now().Add(time.Millisecond * 1))
			myBufLen, err = conn[idx].Read(myBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
			} else {
				if msg := string(myBuf[:myBufLen]); msg == TCP_CONN_QUIT {
					fmt.Println(nickname[idx] + " quit waiting")
					conn[idx].Close()
					ipAddr[idx], udpPort[idx], nickname[idx], conn[idx] = "", "", "", nil
					connCnt--
					return
				}
			}
		}
	}
}

func initCtrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		for i := 0; i < 2; i++ {
			if conn[i] != nil {
				conn[i].Write([]byte(TCP_CONN_REJECT + "server is down."))
			}
		}
		for i := 0; i < 2; i++ {
			if conn[i] != nil {
				conn[i].Close()
			}
		}
		if listener != nil {
			listener.Close()
		}
		os.Exit(0)
	}()
}
