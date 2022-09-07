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

/**
* UDP HEADER
* "0": direct message
	peer: "0"[message]
* "1": direction of new stone of mine
	peer: "1"[row]" "[col]
* "2": give-up message
	peer: "2"[exitting (0 is no, 1 is yes)]
**/

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	TCP_CONN_TYPE string = "tcp"
	UDP_CONN_TYPE string = "udp"
	SERVER_NAME   string = "nsl2.cau.ac.kr"
	SERVER_PORT   string = "50454"

	TCP_CONN_REQUEST  string = "0"
	TCP_CONN_REJECT   string = "1"
	TCP_OPPONENT_DATA string = "2"
	TCP_CONN_QUIT     string = "3"

	UDP_DM        string = "0"
	UDP_STONE_DIR string = "1"
	UDP_GIVE_UP   string = "2"

	GIVEUP_OPTION_GG   string = "0"
	GIVEUP_OPTION_EXIT string = "1"

	GAME_WIN  string = "you win."
	GAME_LOSE string = "you lose."
	GAME_DRAW string = "draw!"

	BUFFER_SIZE int = 1024

	BOARD_SIZE int  = 10
	EMPTY      byte = 0
)

var (
	buffer          []byte         = make([]byte, BUFFER_SIZE)
	tcpConn         net.Conn       = nil
	udpConn         net.PacketConn = nil
	opponentUDPAddr *net.UDPAddr

	myNickname, opponentNickname string

	board                    [10][10]byte
	isGamePlaying            bool = false
	mySymbol, opponentSymbol string
	isMyTurn                 bool
	myTurnCount              int32 = 0

	terminateChan chan bool

	err error
)

func main() {
	initCtrlCHandler()

	if len(os.Args) != 2 {
		fmt.Println("wrong argument")
		return
	} else {
		myNickname = os.Args[1]
	}

	tcpConn, err = net.Dial(TCP_CONN_TYPE, SERVER_NAME+":"+SERVER_PORT)
	if err != nil {
		fmt.Println("no server found.")
		return
	}
	udpConn, err = net.ListenPacket(UDP_CONN_TYPE, ":")
	if err != nil {
		fmt.Println("udp socket init error.")
	}
	tmpStr := strings.Split(udpConn.LocalAddr().String(), ":")
	tcpConn.Write([]byte(TCP_CONN_REQUEST + myNickname + " " + tmpStr[len(tmpStr)-1]))

	buflen, _ := tcpConn.Read(buffer)
	recvMsgHeader, recvMsgBody := string(buffer[:1]), string(buffer[1:buflen])
	if recvMsgHeader == TCP_CONN_REQUEST {
		fmt.Println(recvMsgBody)
		time.Sleep(time.Millisecond * 5)

		buflen, _ := tcpConn.Read(buffer)
		recvMsgHeader, recvMsgBody = string(buffer[:1]), string(buffer[1:buflen])
		if recvMsgHeader == TCP_OPPONENT_DATA {
			setMyPlayerData(recvMsgBody)
		} else if recvMsgHeader == TCP_CONN_REJECT {
			fmt.Println(recvMsgBody)
			return
		}
	} else if recvMsgHeader == TCP_CONN_REJECT {
		fmt.Println(recvMsgBody)
		return
	} else {
		fmt.Println("invalid message")
	}

	// from here, game starts
	isGamePlaying = true
	if mySymbol == "O" {
		fmt.Println(opponentNickname + " joined (" + opponentUDPAddr.String() + "). you play first.")
		isMyTurn = true
		go myTurnTimer()
	} else {
		isMyTurn = false
		fmt.Println(opponentNickname + " play first.")
	}
	printBoard()

	go UDPReceiveHandler()
	go UDPSendHandler()

	terminateChan = make(chan bool)
	defer close(terminateChan)
	<-terminateChan
}

/**
 * keyboard input and send handling goroutine
**/
func UDPSendHandler() {
	var input string
	scanner := *bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		input = scanner.Text()

		if strings.HasPrefix(input, "\\") {
			cmd := input[1:]
			if cmd == "gg" {
				if isGamePlaying {
					isGamePlaying = false
					udpConn.WriteTo([]byte(UDP_GIVE_UP+GIVEUP_OPTION_GG), opponentUDPAddr)
					fmt.Println(GAME_LOSE)
				}
			} else if cmd == "exit" {
				if isGamePlaying {
					fmt.Println(GAME_LOSE)
					isGamePlaying = false
				}
				udpConn.WriteTo([]byte(UDP_GIVE_UP+GIVEUP_OPTION_EXIT), opponentUDPAddr)
				cleanUpAndExit()
			} else if strings.HasPrefix(cmd, "\\ ") {
				dirMsg := strings.TrimPrefix(cmd, "\\ ")
				dir := strings.Split(dirMsg, " ")
				dirInInt := make([]int, 2)
				if len(dir) != 2 {
					fmt.Println("error, must enter x y!")
					continue
				}
				noError := true
				for idx, strDir := range dir {
					if res, err := strconv.Atoi(strDir); err != nil {
						noError = false
					} else {
						dirInInt[idx] = res
					}
				}

				if isGamePlaying {
					if isMyTurn {
						if noError {
							if inRange(dirInInt[0], dirInInt[1]) {
								if board[dirInInt[0]][dirInInt[1]] == EMPTY {
									board[dirInInt[0]][dirInInt[1]] = mySymbol[0]
									udpConn.WriteTo([]byte(UDP_STONE_DIR+dirMsg), opponentUDPAddr)
									atomic.AddInt32(&myTurnCount, 1)
									isMyTurn = false

									printBoard()
									if finished, winner := checkWin(); finished {
										isGamePlaying = false
										switch winner {
										case mySymbol:
											fmt.Println(GAME_WIN)
										case opponentSymbol:
											fmt.Println(GAME_LOSE)
										default:
											fmt.Println(GAME_DRAW)
										}
									}
								} else {
									fmt.Println("error, already used!")
								}
							} else {
								fmt.Println("error, out of bound!")
							}
						} else {
							fmt.Println("error, invalid number format!")
						}
					} else {
						fmt.Println("not your turn.")
					}
				} else {
					continue
				}
			} else {
				fmt.Println("invalid command")
			}
		} else {
			udpConn.WriteTo([]byte(UDP_DM+input), opponentUDPAddr)
		}
	}
}

/**
 * received message handling gorouting
**/
func UDPReceiveHandler() {
	for {
		bufLen, _, _ := udpConn.ReadFrom(buffer)
		if bufLen == 0 {
			break
		}
		recvMsgHeader, recvMsgBody := string(buffer[:1]), string(buffer[1:bufLen])

		if recvMsgHeader == UDP_DM { // dm from opponent
			fmt.Println(opponentNickname + "> " + recvMsgBody)
		} else if recvMsgHeader == UDP_STONE_DIR { // new stone direction of opponent
			splittedMsg := strings.Split(recvMsgBody, " ")
			row, _ := strconv.Atoi(splittedMsg[0])
			col, _ := strconv.Atoi(splittedMsg[1])

			board[row][col] = opponentSymbol[0]
			printBoard()
			if finished, winner := checkWin(); finished {
				isGamePlaying = false
				switch winner {
				case mySymbol:
					fmt.Println(GAME_WIN)
				case opponentSymbol:
					fmt.Println(GAME_LOSE)
				default:
					fmt.Println(GAME_DRAW)
				}
			} else {
				isMyTurn = true
				go myTurnTimer()
			}
		} else if recvMsgHeader == UDP_GIVE_UP { // opponent gg or quit
			if recvMsgBody == GIVEUP_OPTION_GG { // gg
				if isGamePlaying {
					isGamePlaying = false
					fmt.Println(GAME_WIN)
				}
			} else if recvMsgBody == GIVEUP_OPTION_EXIT { // exit, ctrl_c
				if isGamePlaying {
					isGamePlaying = false
					fmt.Println(GAME_WIN)
				}
				cleanUpAndExit()
			} else {
				fmt.Println("invalid command")
			}
		} else {
			fmt.Println("invalid message")
		}
	}
}

/**
 * load count, and sleep for 10 seconds, and check count again.
 * if two counts are same, it means that player didn't send new direction.
 * else, do nothing.
**/
func myTurnTimer() {
	lastMyTurnCnt := atomic.LoadInt32(&myTurnCount)
	time.Sleep(time.Second * 10)
	if atomic.LoadInt32(&myTurnCount) == lastMyTurnCnt && isGamePlaying {
		if isGamePlaying {
			isGamePlaying = false
			udpConn.WriteTo([]byte(UDP_GIVE_UP+GIVEUP_OPTION_GG), opponentUDPAddr)
			fmt.Println("10 seconds timeout!")
			fmt.Println(GAME_LOSE)
		}
	}
}

/**
 * board printing function from omok.go
**/
func printBoard() {
	fmt.Print("   ")
	for i := 0; i < BOARD_SIZE; i++ {
		fmt.Printf("%2d", i)
	}
	fmt.Print("\n  ")
	for i := 0; i < 2*BOARD_SIZE+3; i++ {
		fmt.Print("-")
	}
	fmt.Println()
	for i := 0; i < BOARD_SIZE; i++ {
		fmt.Printf("%d |", i)
		for j := 0; j < BOARD_SIZE; j++ {
			switch board[i][j] {
			case 0:
				fmt.Print(" +")
			case 'O':
				fmt.Print(" O")
			case '@':
				fmt.Print(" @")
			default:
				fmt.Print(" |")
			}
		}
		fmt.Println(" |")
	}
	fmt.Print("  ")
	for i := 0; i < 2*BOARD_SIZE+3; i++ {
		fmt.Print("-")
	}
	fmt.Println()
}

/**
 * check whether new direction is in board
**/
func inRange(row, col int) bool {
	return 0 <= row && row < BOARD_SIZE && 0 <= col && col < BOARD_SIZE
}

/**
 * check whether game is over
 * it returns winner's symbol if game is over and has winner
 * returns two players' symbols if game is tied
**/
func checkWin() (isFinished bool, winner string) {
	// horizontal
	for i := 0; i < BOARD_SIZE; i++ {
		for j := 0; j <= 5; j++ {
			if board[i][j] == board[i][j+1] && board[i][j+1] == board[i][j+2] &&
				board[i][j+2] == board[i][j+3] && board[i][j+3] == board[i][j+4] {
				if board[i][j] == 'O' {
					return true, "O"
				} else if board[i][j] == '@' {
					return true, "@"
				} else {
					continue
				}
			}
		}
	}

	// vertical
	for i := 0; i <= 5; i++ {
		for j := 0; j < BOARD_SIZE; j++ {
			if board[i][j] == board[i+1][j] && board[i+1][j] == board[i+2][j] &&
				board[i+2][j] == board[i+3][j] && board[i+3][j] == board[i+4][j] {
				if board[i][j] == 'O' {
					return true, "O"
				} else if board[i][j] == '@' {
					return true, "@"
				} else {
					continue
				}
			}
		}
	}

	// right-downward
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5; j++ {
			if board[i][j] == board[i+1][j+1] && board[i+1][j+1] == board[i+2][j+2] &&
				board[i+2][j+2] == board[i+3][j+3] && board[i+3][j+3] == board[i+4][j+4] {
				if board[i][j] == 'O' {
					return true, "O"
				} else if board[i][j] == '@' {
					return true, "@"
				} else {
					continue
				}
			}
		}
	}

	// left-downward
	for i := 4; i < BOARD_SIZE; i++ {
		for j := 0; j <= 5; j++ {
			if board[i][j] == board[i-1][j+1] && board[i-1][j+1] == board[i-2][j+2] &&
				board[i-2][j+2] == board[i-3][j+3] && board[i-3][j+3] == board[i-4][j+4] {
				if board[i][j] == 'O' {
					return true, "O"
				} else if board[i][j] == '@' {
					return true, "@"
				} else {
					continue
				}
			}
		}
	}

	// tie or not finished
	for i := 0; i < BOARD_SIZE; i++ {
		for j := 0; j < BOARD_SIZE; j++ {
			if board[i][j] == EMPTY {
				return false, ""
			}
		}
	}
	return true, "O@"
}

/**
 * set player's data which were received from server
**/
func setMyPlayerData(msg string) {
	fragment := strings.Split(msg, " ")
	opponentNickname, mySymbol = fragment[1], fragment[2]
	opponentUDPAddr, _ = net.ResolveUDPAddr(UDP_CONN_TYPE, fragment[0])
	if mySymbol == "O" {
		opponentSymbol = "@"
	} else {
		opponentSymbol = "O"
	}
}

func initCtrlCHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cleanUpAndExit()
	}()
}

func cleanUpAndExit() {
	if tcpConn != nil {
		tcpConn.Write([]byte(TCP_CONN_QUIT))
	}
	if udpConn != nil {
		if isGamePlaying {
			fmt.Println(GAME_LOSE)
			isGamePlaying = false
		}
		udpConn.WriteTo([]byte(UDP_GIVE_UP+GIVEUP_OPTION_EXIT), opponentUDPAddr)
		udpConn.Close()
	}

	fmt.Println("Bye~")
	if terminateChan != nil {
		terminateChan <- true
	} else {
		os.Exit(0)
	}
}
