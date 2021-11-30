package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	CONN_HOST = "localhost"
	CONN_TYPE = "tcp"
	LOG_FILE  = "logs.txt"
)

var CONN_PORT = "8989"

func main() {
	if len(os.Args) > 1 {
		if len(os.Args) == 2 {
			CONN_PORT = os.Args[1]
		} else {
			fmt.Println("[USAGE]: ./TCPChat $port")
			os.Exit(1)
		}
	}
	if _, err := os.Stat(LOG_FILE); errors.Is(err, os.ErrNotExist) {
		os.Create(LOG_FILE)
	}
	logs, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	errCheck(err)
	defer logs.Close()

	l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+CONN_PORT)
	errCheck(err)
	defer l.Close()
	fmt.Println("Server started on: " + CONN_HOST + ":" + CONN_PORT)

	connMap := &sync.Map{} // uses mutexes

	for {
		conn, err := l.Accept()
		errCheck(err)
		connMap.Store(conn.RemoteAddr().String(), conn)
		go handleRequest(conn, connMap, logs)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn, connMap *sync.Map, logs *os.File) {
	fmt.Printf("[CONNECTED]: %s\n", conn.RemoteAddr().String())

	// 10 connections max at all times (but briefly 11, as seen here)
	if getMapLength(connMap) > 10 {
		conn.Write([]byte("Chat is at full capacity, sorry! Try again later.\n"))
		fmt.Printf("[DISCONNECTED]: %s\n", conn.RemoteAddr().String())
		conn.Close()
		connMap.Delete(conn.RemoteAddr().String())
		return
	}
	dat, _ := ioutil.ReadFile("intro.txt")
	name := make([]byte, 16)
	conn.Write([]byte(dat))
invalid:
	conn.Read(name)
	if name[0] == 10 || name[0] == 32 {
		conn.Write([]byte("Name cannot be empty.\n"))
		goto invalid
	}
	name = []byte(strings.ReplaceAll(strings.ReplaceAll(string(name), "\n", ""), string(0), ""))
	conn.Write([]byte("Past messages will now be loaded.\n"))
	time.Sleep(time.Second * 2)

	data, err := os.ReadFile(LOG_FILE)
	errCheck(err)

	conn.Write(data)

	connMap.Range(func(key, value interface{}) bool {
		if c, ok := value.(net.Conn); ok {
			c.Write([]byte("[" + string(name) + " JOINED]\n"))
		}
		return true
	})
	conn.Write([]byte("Type .quit to exit.\n"))
	for {
		netData, err := bufio.NewReader(conn).ReadString('\n')
		errCheck(err)
		msg := strings.TrimSpace(netData)
		if msg == ".quit" {
			break
		}

		line := "[" + time.Now().Format("02.01.2006 15:04:05") + " (+03:00)] [" + string(name) + "]: " + msg
		connMap.Range(func(key, value interface{}) bool {
			if c, ok := value.(net.Conn); ok {
				if len(msg) != 0 {
					c.Write([]byte(line + "\n"))
				}
			}
			return true
		})
		_, err = logs.WriteString(line + "\n")
		errCheck(err)
	}

	fmt.Printf("[DISCONNECTED]: %s\n", conn.RemoteAddr().String())
	connMap.Range(func(key, value interface{}) bool {
		if c, ok := value.(net.Conn); ok {
			c.Write([]byte("[" + string(name) + " LEFT]\n"))
		}
		return true
	})
	conn.Close()
	connMap.Delete(conn.RemoteAddr().String())
}

// Calculates the length of the sync map.
func getMapLength(connMap *sync.Map) int {
	var length int
	connMap.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return length
}

func errCheck(err error) {
	if err != nil {
		log.Fatalf("An error occurred.\n%s", err.Error())
	}
}
