package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
)

type Server struct {
	address  string
	listener net.Listener
	buffer   chan []byte

	stopOnes sync.Once
	wg       sync.WaitGroup
	stopChan chan struct{}
}

func NewServer(port, host string) *Server {
	return &Server{
		address:  host + ":" + port,
		buffer:   make(chan []byte, 10),
		stopChan: make(chan struct{}, 1),
	}
}

func (serv *Server) Start() error {
	l, err := net.Listen("tcp", serv.address)
	if err != nil {
		log.Println("Failed to create TCP Listener. Error:", err)
		return err
	}
	serv.listener = l
	log.Println("Listening on:", serv.address)
	go serv.acceptLoop()
	<-serv.stopChan
	return nil
}

func (serv *Server) Stop() {
	serv.stopOnes.Do(func() {
		if serv.listener != nil {
			serv.listener.Close()
		}
	})
}

func (serv *Server) acceptLoop() {
	defer func() {
		serv.wg.Wait()
		close(serv.buffer)
		close(serv.stopChan)
		log.Println("Server stopped")
	}()
	for {
		conn, err := serv.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Connection is closed. Error:", err)
				return
			}
			log.Println("Accept error:", err)
			continue
		}
		log.Println("New connection established successfully:", conn.RemoteAddr().String())
		serv.wg.Add(1)
		go serv.readLoop(conn)
	}
}

func (serv *Server) readLoop(conn net.Conn) {
	defer serv.wg.Done()
	defer conn.Close()
	for {
		buf := make([]byte, 1024)
		size, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("No more input is availiable")
				return
			}
			log.Println("Failed to read:", err)
			return
		}
		if size == 0 {
			continue
		}
		data := bytes.TrimSpace(buf[:size])
		if bytes.Equal(data, []byte("stop")) {
			log.Println("Stop command received")
			serv.Stop()
			conn.Write([]byte("Stopping server"))
			return
		}
		select {
		case serv.buffer <- data:
		default:
			log.Println("Buffer is full, dropping message")
		}

		_, err = conn.Write(append([]byte("Received data:"), append(data, '\n')...))
		if err != nil {
			log.Println("Failed to write:", err)
			return
		}
	}
}

func main() {
	port := flag.Int("port", 8080, "Port for connection acception.")
	host := flag.String("host", "localhost", "Host or IP")
	flag.Parse()

	serv := NewServer(strconv.Itoa(*port), *host)
	go func() {
		for data := range serv.buffer {
			log.Println("Recieved data:", data)
		}
	}()
	if err := serv.Start(); err != nil {
		log.Fatal(err)
	}
}
