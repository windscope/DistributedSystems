// Implementation of a MultiEchoServer. Students should write their code in this file.

package bserver

import (
    "bufio"
    "fmt"
    "net"
    "strconv"
    "errors"
)

type multiEchoServer struct {
    ServerClosed bool // check whether the multiEcho server is closed
    listener net.Listener
    resMan *resourceManager // manage all resource for the multiEchoServer
}

func New() MultiEchoServer {
    return &multiEchoServer{ServerClosed: false, listener: nil, resMan: NewResourceManager()}
}

func (mes *multiEchoServer) Start(port int) error {
    if mes.ServerClosed {
        return errors.New("Server Error: Start server when server closed")
    }
    l, err := net.Listen("tcp", ":" + strconv.Itoa(port))
    mes.listener = l // if connect successful, assign the listener with the server
    if err != nil {
        return errors.New(fmt.Sprintf("Server Error: listen: %s", err))
    }
    go mes.resMan.Start() // start the resource manager
    go mes.handleClients() // handling all the clients
    return nil
}

func (mes *multiEchoServer) Close() {
    mes.ServerClosed = true
    mes.resMan.Close()
    mes.listener.Close()
}

func (mes *multiEchoServer) Count() int {
    mes.resMan.countReq <- struct{}{}
    return <- mes.resMan.countResp
}

func (mes *multiEchoServer) handleClients() {
    for i := 0; true; i++ { //infinite loop with each time get unique i
        conn, err := mes.listener.Accept()
        if err != nil {
            mes.resMan.done <- struct{}{} // close the resourceManager
            return
        }
        client := &clientHandler{i, conn, make(chan string, 950), make(chan struct{})}
        mes.resMan.addReq <- client // let resourceManager to register the newly established client
    }
}

type resourceManager struct {
    clients map[int]*clientHandler // map of all existing client handler
    messageChan chan string // messageChan used to sync all clients received message
    addReq chan *clientHandler // request chan for adding new clientHandler
    deleteReq chan *clientHandler // request chan for remove closed clientHandlers
    countReq chan struct{} // request chan for counting all exisiting clientHandlers
    countResp chan int // response chan for all counting exisiting clientHandlers
    done chan struct{} // done channel for closing all goroutines
}

func NewResourceManager() *resourceManager {
    return &resourceManager{
        clients: make(map[int]*clientHandler),
        messageChan: make(chan string),
        addReq: make(chan *clientHandler),
        deleteReq: make(chan *clientHandler),
        countReq: make(chan struct{}),
        countResp: make(chan int),
        done: make(chan struct{})}
}

func (rm *resourceManager) Start() {
    for {
        select {
        case <- rm.done:
            rm.clients = make(map[int]*clientHandler)
            return
        case ch := <- rm.addReq:
            rm.clients[ch.cid] = ch // register the client upon req
            go ch.startRecv(rm) // continuously receive from client conn
            go ch.startSend() // continuously send to client conn once message channel has message
        case ch := <- rm.deleteReq:
            delete(rm.clients, ch.cid) // delete the client upon req
            ch.Close()
        case <- rm.countReq: rm.countResp <- len(rm.clients) // recv count request, return the count
        case s := <- rm.messageChan: rm.broadcast(s) // broadcast the received message
        }
    }
}
func (rm *resourceManager) Close() {
    rm.done <- struct{}{}
}
func (rm *resourceManager) broadcast(message string) {
    for _, ch := range rm.clients {
        ch.writeToBuf(message)
    }
}

type clientHandler struct {
    cid int
    conn net.Conn
    responseChan chan string
    done chan struct{}
}

func (ch *clientHandler) Close() {
    defer ch.conn.Close()
    defer close(ch.responseChan)
    ch.done <- struct{}{}
}

func (ch *clientHandler) startRecv(rm *resourceManager) {
    defer func() {rm.deleteReq <- ch}() // delete the client handler if exit this function
    input := bufio.NewScanner(ch.conn)
    for input.Scan() {
        rm.messageChan <- input.Text();
    }
}

func (ch *clientHandler) writeToBuf(s string) {
    select {
    case ch.responseChan <- s:
    default: // buffer is full, ignoring the incoming echos
    }
}

func (ch *clientHandler) startSend() {
    for {
        select {
        case s := <-ch.responseChan: fmt.Fprint(ch.conn, fmt.Sprintf("%s%s\n", s, s))
        case <-ch.done: return
        }
    }
}
