# GoChat
A secure Go chat client-server end-to-end with RSA 2048 and AES 256 using WebSocket

## How it works

Start the server for the clients that will connect, then when 2 or more clients are connected you can communicate each other.

When you start a conversation the AES key to encrypt the messages is passed between client, through the server, but the server can't decrypt the message, thanks to RSA property, becuase it doesn't own the private key of the receiver.

After that, every message sent by the 2 clients is end-to-end, and no one a part the two who exchanged the keys can decrypt the messages.

## Guide step-by-step

### Installation

```
$ git clone https://github.com/0x5eba/GoChat.git
$ cd GoChat
$ ./init.sh
```

### Server

To start the server `sudo ./chat s` and you should see

```INFO[0000] Starting server on port 3141```

### Client

To start the server `sudo ./chat c [name]` for example `sudo ./chat c carl`

To send a message to another client use the command `chat`

For example if Pippo want to send a message to Pluto

```
>>> chat
To whom do you want to write?
   pippo
 ❯ pluto
> Hello Pluto
```

Then Pluto will receive

```
INFO[0001] [pippo] : Hello Pluto
```
