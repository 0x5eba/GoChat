package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	b64 "encoding/base64"

	_ "github.com/go-sql-driver/mysql"

	network "github.com/0x5eba/GoChat/protobufs/build/network"
	upnp "github.com/NebulousLabs/go-upnp"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// ConnectionStore handles peer messaging
type ConnectionStore struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	readOther chan []byte
	store     *ConnectionStore
	wg        sync.WaitGroup
	isOpen    bool
	rsaKey    []byte
	username  string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var currentStore *ConnectionStore

func getPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	peers := []string{}

	for k := range currentStore.clients {
		// fullIP := k.conn.RemoteAddr().String()
		// ip := strings.Split(fullIP, ":")[0]
		peers = append(peers, string(k.rsaKey)+" -> "+k.username)
	}

	resp, _ := json.Marshal(peers)
	w.Write(resp)
}

// TraverseNat opens the port passed as an arguemnt and returns
// ip:port in a string. Required manaually closing port
func TraverseNat(port uint16) (string, error) {
	// Connect to router
	d, err := upnp.Discover()
	if err != nil {
		return "", err
	}

	ip, err := d.ExternalIP()
	if err != nil {
		return "", err
	}

	// Remove forwarding map and ignore error in case it doesn't exist
	d.Clear(port)
	// Open the port
	err = d.Forward(port, "GO chat")

	return fmt.Sprintf("%s:%d", ip, port), nil
}

// StartPeerServer creates an HTTP server that replies with known peers
func StartPeerServer() {
	http.HandleFunc("/peers", getPeers)
	log.Fatal(http.ListenAndServe(":80", nil))
}

// StartServer start server
func StartServer(port string) {
	store := &ConnectionStore{
		clients:    make(map[*client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *client),
		unregister: make(chan *client),
	}
	currentStore = store

	go store.run()
	log.Info("Starting server on port ", port)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Info("New connection")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error(err)
			return
		}

		c := client{
			conn:      conn,
			send:      make(chan []byte, 256),
			readOther: make(chan []byte, 256),
			store:     store,
			isOpen:    true,
		}

		store.register <- &c

		go c.read()
		go c.write()
	})

	// Server that allows peers to connect
	go http.ListenAndServe(port, nil)

	select {}
}

// run is the event handler to update the ConnectionStore
func (cs *ConnectionStore) run() {
	for {
		select {
		// A new client has registered
		case client := <-cs.register:
			client.wg.Add(1)
			client.wg.Done()
			cs.clients[client] = true

		// A client has quit, check if it exisited and delete it
		case client := <-cs.unregister:
			if _, ok := cs.clients[client]; ok {
				// Don't close the channel till we're done responding to avoid log.Errors
				client.wg.Wait()
				delete(cs.clients, client)
				close(client.send)
			}
		}
	}
}

type UserServer struct {
	b64Rsa   string `json:"b64Rsa"`
	username string `json:"username"`
}

// read reads data from the socket and handles it
func (c *client) read() {
	// Unregister if the node dies
	defer func() {
		log.Info("Client died")
		c.store.unregister <- c
		c.isOpen = false
		c.conn.Close()
	}()

	for {
		msgType, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Error(err)
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		pb := network.Envelope{}
		err = proto.Unmarshal(msg, &pb)
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			switch pb.GetType() {

			case network.Envelope_MESSAGE:

				message := network.Message{}
				err = proto.Unmarshal(pb.GetData(), &message)
				if err != nil {
					log.Error(err)
					return
				}

				db, err := sql.Open("mysql", "root:test123@tcp(127.0.0.1:3306)/")
				if err != nil {
					log.Print(err.Error())
				}
				defer db.Close()

				db.Exec("USE Server;")

				// TODO bho non va e che cazzo ne so io
				// if message.GetType() != network.Message_RSA_SERVER {
				// 	b64Sender := b64.URLEncoding.EncodeToString(message.GetSender())
				// 	b64Destination := b64.URLEncoding.EncodeToString(message.GetDestination())
				// 	b64Data := b64.URLEncoding.EncodeToString(message.GetData())

				// 	insert, err := db.Query("INSERT INTO History (rsa_sender, rsa_receiver, data, type_message) VALUES ( ?, ?, ?, ? )", b64Sender, b64Destination, b64Data, message.GetType())
				// 	if err != nil {
				// 		log.Error(err.Error())
				// 	}
				// 	defer insert.Close()
				// }

				switch message.GetType() {
				case network.Message_RSA_SERVER:
					user := network.User{}
					err = proto.Unmarshal(message.GetData(), &user)
					if err != nil {
						log.Error(err)
						return
					}

					b64Rsa := b64.URLEncoding.EncodeToString(user.GetRsaKey())

					// check if the rsa already exist in the db
					var resQuery UserServer
					err = db.QueryRow("SELECT * FROM Clients where rsa_client = ?", b64Rsa).Scan(&resQuery.b64Rsa, &resQuery.username)

					// se non esiste l'rsa allora crea l'utente
					if err != nil {
						db.Exec("USE Server;")

						insert, err := db.Query("INSERT INTO Clients VALUES ( ?, ? )", b64Rsa, user.GetUsername())
						if err != nil {
							log.Error(err.Error())
						}
						defer insert.Close()

						c.rsaKey = user.GetRsaKey()
						c.username = user.GetUsername()

						log.Info("New User: ", user.GetUsername())
					} else {
						log.Error("User already exist")
					}

				case network.Message_AES_CLIENT, network.Message_MESSAGE:
					for k := range currentStore.clients {
						if reflect.DeepEqual(k.rsaKey, message.GetDestination()) == true {
							// send back the envelope arrived to sever

							if message.GetType() == network.Message_MESSAGE {
								log.Info("Data Message: ", string(message.GetData()))
							}

							if message.GetType() == network.Message_AES_CLIENT {
								log.Info("Sender: ", b64.URLEncoding.EncodeToString(message.GetSender()))
								log.Info("Receiver: ", b64.URLEncoding.EncodeToString(message.GetDestination()))
								log.Info("Exchange AES Keys: ", string(message.GetData()))
							}

							k.send <- msg
							break
						}
					}
				}
			}
		}()
	}
}

// write checks the channel for data to write and writes it to the socket
func (c *client) write() {
	for {
		toWrite := <-c.send

		_ = c.conn.WriteMessage(websocket.BinaryMessage, toWrite)
	}
}
