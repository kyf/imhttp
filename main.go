package main

import (
	clist "container/list"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Monitor struct {
	ActiveNum       int
	ActiveNumLocker sync.Mutex
}

func (m *Monitor) Add() {
	m.ActiveNumLocker.Lock()
	defer m.ActiveNumLocker.Unlock()

	m.ActiveNum += 1
}

func (m *Monitor) Done() {
	m.ActiveNumLocker.Lock()
	defer m.ActiveNumLocker.Unlock()

	if m.ActiveNum > 0 {
		m.ActiveNum -= 1
	}
}

type OlMap struct {
	maps    map[string]*clist.List
	locker  sync.Mutex
	msgChan chan string
}

func NewOM() *OlMap {
	return &OlMap{maps: make(map[string]*clist.List), msgChan: make(chan string, 1)}
}

func (o *OlMap) Reg(addr string) {
	o.locker.Lock()
	defer o.locker.Unlock()

	o.maps[addr] = clist.New()
}

func (o *OlMap) Unreg(addr string) {
	o.locker.Lock()
	defer o.locker.Unlock()

	delete(o.maps, addr)
}

func (o *OlMap) Run() {
	for {
		select {
		case msg := <-o.msgChan:
			go func() {
				for _, stack := range o.maps {
					stack.PushBack(msg)
				}
			}()
		}
	}
}

var (
	m       = &Monitor{}
	onlines = NewOM()
)

func handleConn(conn net.Conn) {
	defer conn.Close()
	m.Add()
	addr := conn.RemoteAddr().String()
	onlines.Reg(addr)

	var buf []byte = make([]byte, 2048)
	for {
		num, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			fmt.Printf("conn.Read err:%v\n", err)
			goto Exit
		}
		conn.Write([]byte("HTTP/1.1 200 OK\r\n"))
		conn.Write([]byte("Date: Tue, 03 May 2016 02:45:43 GMT\r\n"))
		conn.Write([]byte("Content-Length: 2\r\n"))
		conn.Write([]byte("Content-Type: text/plain; charset=utf-8\r\n"))
		conn.Write([]byte("\r\n"))
		conn.Write([]byte("11"))
		if num > 0 {
			data := string(buf[:num])
			data_list := strings.Split(data, "\r\n\r\n")
			if len(data_list) > 1 {
				body := data_list[1]
				if len(body) > 0 {
					onlines.msgChan <- body
				}
			}
		}

		if me, ok := onlines.maps[addr]; ok {
			result := make([]string, 0, me.Len())
			for i := 0; i < me.Len(); i++ {
				m1 := me.Front()
				me.Remove(m1)
				if msg, ok := m1.Value.(string); ok {
					result = append(result, msg)
				}
			}
			_result, err := json.Marshal(result)
			if err == nil {
				conn.Write(_result)
			} else {
				fmt.Printf("json.Marshal err:%v\n", err)
			}
		}

	}

Exit:

	onlines.Unreg(addr)
	m.Done()
}

func main() {
	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		fmt.Printf("net.Listen err:%v\n", err)
		os.Exit(1)
	}

	defer listener.Close()

	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for {
			select {
			case <-ticker.C:
				fmt.Println("Current Active Number is ", m.ActiveNum)
			}
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("net.Listen err:%v\n", err)
			os.Exit(1)
		}

		go handleConn(conn)
	}
}
