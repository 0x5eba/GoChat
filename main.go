package main

import (
	"fmt"
	"os"
	"strconv"

	// audio "./streaming/audio"

	"database/sql"

	"github.com/0x5eba/GoChat/SimpleChat/client"
	"github.com/0x5eba/GoChat/SimpleChat/server"
	_ "github.com/go-sql-driver/mysql"
	"github.com/urfave/cli"
)

const (
	PORT = 3141
)

func CreateDB(isClient bool) {
	db, err := sql.Open("mysql", "root:test123@tcp(127.0.0.1:3306)/")
	if err != nil {
		fmt.Println(err.Error())
	}
	defer db.Close()

	if isClient == true {
		db.Exec("DROP DATABASE Client;")
		db.Exec("CREATE DATABASE Client;")
		db.Exec("ALTER DATABASE Client CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;")
		db.Exec("USE Client;")
		db.Exec("SET NAMES utf8mb4;")
		db.Exec("CREATE TABLE Chats(rsa_key varchar(700) NOT NULL, aes_key varchar(700) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, IV_key varchar(700) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, PRIMARY KEY (rsa_key)) DEFAULT CHARSET=utf8mb4;")
	} else {
		db.Exec("DROP DATABASE Server;")
		db.Exec("CREATE DATABASE Server;")
		db.Exec("ALTER DATABASE Server CHARACTER SET utf8mb4 COLLATE utf8_unicode_ci;")
		db.Exec("USE Server;")
		db.Exec("SET NAMES utf8mb4;")
		db.Exec("CREATE TABLE Clients(rsa_client varchar(700) NOT NULL, username varchar(30), PRIMARY KEY (rsa_client)) DEFAULT CHARSET=utf8mb4;")
		// PRIMARY KEY (ts, rsa_sender, rsa_receiver)
		// db.Exec("CREATE TABLE History(rsa_sender varchar(700), rsa_receiver varchar(700), data varchar(700) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci, type_message varchar(20), ts TIMESTAMP DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (ts)) DEFAULT CHARSET=utf8mb4;")
	}
}

func main() {
	app := cli.NewApp()
	app.Version = "1.0.0 pre-alpha"
	app.Commands = []cli.Command{
		{
			Name:    "client",
			Usage:   "client [username]",
			Aliases: []string{"client", "c"},
			Action: func(c *cli.Context) error {

				username := c.Args().Get(0)

				if _, err := os.Stat("./client/RsaKeys" + username); os.IsNotExist(err) {
					os.MkdirAll("./client/RsaKeys"+username, os.ModePerm)
					client.GenerateRSAKey(username)
				}

				CreateDB(true)
				client.StartClient("localhost", username)

				return nil
			},
		},

		{
			Name:    "server",
			Usage:   "server",
			Aliases: []string{"server", "s"},
			Action: func(c *cli.Context) error {

				go server.StartPeerServer()
				// server.TraverseNat(PORT)
				CreateDB(false)
				server.StartServer(strconv.Itoa(PORT))

				return nil
			},
		},
	}

	app.Run(os.Args)
}
