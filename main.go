package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type client struct {
	conn     net.Conn
	nick     string
	room     *room
	commands chan<- command
}

type commandID int

const (
	CMD_JOIN commandID = iota
	CMD_ROOMS
	CMD_MSG
	CMD_QUIT
	CMD_NICK
)

type command struct {
	id     commandID
	client *client
	args   []string
	heure  string
}

type room struct {
	name     string
	members  map[net.Addr]*client
	messages []string
}

type server struct {
	rooms    map[string]*room
	commands chan command
}

func (c *client) readInput(s *server, l *[]string) {
	for {
		if c.nick != "" {
			msg, err := bufio.NewReader(c.conn).ReadString('\n')
			if err != nil {
				log.Printf("client has left the chat: %s", c.conn.RemoteAddr().String())
				var a []string
				for _, v := range *l {
					if v != c.nick {
						a = append(a, v)
					}
				}
				*l = a
				if c.room != nil {
					oldRoom := s.rooms[c.room.name]
					delete(s.rooms[c.room.name].members, c.conn.RemoteAddr())
					oldRoom.broadcast(c, fmt.Sprintf(" has left the room"))
				}
				c.conn.Close()
				return
			}

			msg = strings.Trim(msg, "\r\n")

			args := strings.Split(msg, " ")
			cmd := strings.TrimSpace(args[0])

			switch cmd {
			case "/join":
				c.commands <- command{
					id:     CMD_JOIN,
					client: c,
					args:   args,
					heure:  "[" + time.Now().Format("2006-01-02 15:4:5") + "]",
				}
			case "/rooms":
				c.commands <- command{
					id:     CMD_ROOMS,
					client: c,
					heure:  "[" + time.Now().Format("2006-01-02 15:4:5") + "]",
				}
			case "/quit":
				c.commands <- command{
					id:     CMD_QUIT,
					client: c,
					heure:  "[" + time.Now().Format("2006-01-02 15:4:5") + "]",
				}
			case "/name":
				c.commands <- command{
					id:     CMD_NICK,
					client: c,
					args:   args,
					heure:  "[" + time.Now().Format("2006-01-02 15:4:5") + "]",
				}
			default:
				c.commands <- command{
					id:     CMD_MSG,
					client: c,
					args:   args,
					heure:  "[" + time.Now().Format("2006-01-02 15:4:5") + "]",
				}
			}
		}
	}
}

func (c *client) err(err error) {
	c.conn.Write([]byte("err: " + err.Error() + "\n"))
}

func (c *client) msg(msg string) {

	c.conn.Write([]byte(msg + "\n"))
}

func (r *room) broadcastmsg(sender *client, msg string) {
	for addr, m := range r.members {
		if sender.conn.RemoteAddr() != addr {
			m.conn.Write([]byte("[" + time.Now().Format("2006-01-02 15:4:5") + "]" + "[" + sender.nick + "]" + msg + "\n"))
		}
	}
	r.messages = append(r.messages, "["+time.Now().Format("2006-01-02 15:4:5")+"]"+"["+sender.nick+"]"+msg+"\n")
}

func (r *room) broadcast(sender *client, msg string) {
	for addr, m := range r.members {
		if sender.conn.RemoteAddr() != addr {
			m.conn.Write([]byte(sender.nick + msg + "\n"))
		}
	}
	r.messages = append(r.messages, sender.nick+msg+"\n")
}

func newServer() *server {
	return &server{
		rooms:    make(map[string]*room),
		commands: make(chan command),
	}
}

func (s *server) run(l *[]string) {
	for cmd := range s.commands {
		switch cmd.id {
		case CMD_JOIN:
			s.join(cmd.client, cmd.args)
		case CMD_ROOMS:
			s.listRooms(cmd.client)
		case CMD_NICK:
			nick(cmd.client, l, s, cmd.args)
		case CMD_MSG:
			s.msg(cmd.client, cmd.args)
		case CMD_QUIT:
			s.quit(cmd.client, l)
		}
	}
}
func nick(c *client, l *[]string, s *server, args []string) {
	if len(args) < 2 {
		c.msg("New name is required. usage: /name PSEUDO")
		return
	}
	var a []string
	for _, v := range *l {
		if v != c.nick {
			a = append(a, v)
		}
	}
	*l = a
	exist := false
	for _, v := range *l {
		if v == args[1] {
			exist = true
		}
	}
	if exist {
		c.msg("[The PSEUDO already existe try another one: ]")
		return
	} else {
		*l = append(*l, args[1])
		c.nick = args[1]
		c.msg("[SUCCESS]")
	}
}

func (s *server) newClient(conn net.Conn) *client {
	log.Printf("new client has joined: %s", conn.RemoteAddr().String())

	return &client{
		conn:     conn,
		nick:     "",
		commands: s.commands,
	}
}

func (s *server) join(c *client, args []string) {
	if len(args) < 2 {
		c.msg("room name is required. usage: /join ROOM_NAME")
		return
	}

	roomName := args[1]

	r, ok := s.rooms[roomName]
	if !ok {
		r = &room{
			name:    roomName,
			members: make(map[net.Addr]*client),
		}
		s.rooms[roomName] = r
	}
	r.members[c.conn.RemoteAddr()] = c

	s.quitCurrentRoom(c)
	c.room = r

	r.broadcast(c, fmt.Sprintf(" joined the room"))

	for _, r := range r.messages {
		c.conn.Write(([]byte(r)))
	}
}

func (s *server) listRooms(c *client) {
	var rooms []string
	for name := range s.rooms {
		rooms = append(rooms, name)
	}

	c.msg(fmt.Sprintf("available rooms: %s", strings.Join(rooms, ", ")))
}

func (s *server) msg(c *client, args []string) {
	if len(s.rooms) == 0 {
		c.msg("First join or creat a room, usage: /join Nameroom")
		return
	}
	msg := strings.Join(args, " ")
	if c.room != nil {
		c.room.broadcastmsg(c, ": "+msg)
	} else {
		c.msg("first join or create a room, usage: /join ")
	}
}

func (s *server) quit(c *client, l *[]string) {
	log.Printf("client has left the chat: %s", c.conn.RemoteAddr().String())
	var a []string
	for _, v := range *l {
		if v != c.nick {
			a = append(a, v)
		}
	}
	*l = a
	s.quitCurrentRoom(c)

	c.msg("sad to see you go =(")
	c.conn.Close()
}

func (s *server) quitCurrentRoom(c *client) {
	if c.room != nil {
		oldRoom := s.rooms[c.room.name]
		delete(s.rooms[c.room.name].members, c.conn.RemoteAddr())
		oldRoom.broadcast(c, fmt.Sprintf(" has left the room"))
	}
}

func newconnection(c *client, l *[]string, s *server) {
	c.conn.Write([]byte(
		`Welcome to TCP-Chat!
    _nnnn_
   dGGGGMMb
  @p~qp~~qMb
  M|@||@) M|
  @,----.JM|
 JS^\__/  qKL
dZP        qKRb
dZP          qKKb
fZP            SMMb
HZM            MMMM
FqM            MMMM
__| ".        |\dS"qML
|    ` + "`" + `.       | ` + "`" + `' \Zq
_)     \.___.,|     .'
\____  )MMMMMP|   .'
     ` + "`" + `-'       ` + "`" + `--'
[ENTER YOUR NAME]:`))
	msg, err := bufio.NewReader(c.conn).ReadString('\n')
	msg = strings.Trim(msg, "\r\n")

	args := strings.Split(msg, " ")
	if err != nil {
		log.Printf("client has left the chat: %s", c.conn.RemoteAddr().String())
		var a []string
		for _, v := range *l {
			if v != c.nick {
				a = append(a, v)
			}
		}
		*l = a
		if c.room != nil {
			oldRoom := s.rooms[c.room.name]
			delete(s.rooms[c.room.name].members, c.conn.RemoteAddr())
			oldRoom.broadcast(c, fmt.Sprintf(" has left the room"))
		}
		c.conn.Close()
		return
	}
	var dontexiste = false
	for _, r := range *l {
		if r == msg {
			c.conn.Write([]byte("[The PSEUDO already existe try another one: ]"))
			dontexiste = true
			break
		}
	}
	for args[0] == "" || dontexiste {
		if args[0] == "" {
			c.conn.Write([]byte("[The PSEUDO is nul enter another one: ]"))
		}
		dontexiste = false
		msg, err = bufio.NewReader(c.conn).ReadString('\n')
		msg = strings.Trim(msg, "\r\n")

		args = strings.Split(msg, " ")
		if err != nil {
			log.Printf("client has left the chat: %s", c.conn.RemoteAddr().String())
			var a []string
			for _, v := range *l {
				if v != c.nick {
					a = append(a, v)
				}
			}
			*l = a
			if c.room != nil {
				oldRoom := s.rooms[c.room.name]
				delete(s.rooms[c.room.name].members, c.conn.RemoteAddr())
				oldRoom.broadcast(c, fmt.Sprintf(" has left the room"))
			}
			c.conn.Close()
			return
		}
		for _, r := range *l {
			if r == msg {
				c.conn.Write([]byte("[The PSEUDO already existe try another one: ]"))
				dontexiste = true
				break
			}
		}
	}

	*l = append(*l, msg)
	c.nick = msg
	c.conn.Write([]byte("How to use:\nTo creat or join a room: [/join <name room>]\nTo check the avaiable rooms: [/rooms]\nTo leave: [/quit]\n"))
}

func main() {
	var l []string
	nmbr := 0
	port := "8989"
	if len(os.Args) == 2 {
		port = os.Args[1]
	}
	if len(os.Args) > 2 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(0)
	}
	s := newServer()
	go s.run(&l)
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("unable to start server: %s", err.Error())
	}

	defer listener.Close()
	log.Printf("server started on localhost:%s", port)

	for {
		if len(l) < 9 {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("failed to accept connection: %s", err.Error())
				continue
			}
			c := s.newClient(conn)
			go newconnection(c, &l, s)
			go c.readInput(s, &l)
		} else {
			conn, _ := listener.Accept()
			nmbr++
			c := &client{
				conn:     conn,
				nick:     "",
				commands: s.commands,
			}
			c.conn.Write([]byte("Erreur number max\n"))
			c.conn.Close()
			log.Printf("a client try to joined but limeted: %s", conn.RemoteAddr().String())
		}

	}
}
