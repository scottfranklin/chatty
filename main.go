// A simple chat relay that forwards incoming messages to everyone in a group.
// Mostly copied from the example code in github.com/mattn/go-xmp.
package main


import (
    "fmt"
    "flag"
    "github.com/mattn/go-xmpp"
    "github.com/mattn/go-iconv"
    "github.com/peterbourgon/diskv"
    "log"
    "os"
    "strings"
    "time"
)

var server   = flag.String("server", "talk.google.com:443", "server")
var username = flag.String("username", "", "username")
var password = flag.String("password", "", "password")
var adduser = flag.String("adduser", "", "adduser")
var rmuser = flag.String("rmuser", "", "rmuser")

var db = diskv.New(diskv.Options{
        BasePath:     "chat_users",
        Transform:    func(s string) []string { return []string{} },
        CacheSizeMax: 1024 * 1024, // 1MB
    })

var users map[string]string


func handle(client *xmpp.Client, chat xmpp.Chat) {

    user := strings.Split(chat.Remote, "/")[0]

    _, ok := users[user]
    if !ok {
        log.Printf("User %s not found.", user)
        return
    }

    response := fmt.Sprintf("[%s] %s", users[user], chat.Text)
    self := false
    others := true

    command_bits := strings.SplitN(chat.Text, " ", 2)
    command := command_bits[0]
    arg := ""
    if len(command_bits) > 1 {
        arg = command_bits[1]
    }
    if command[0] == '/' {
        switch command {
        case "/whois":
            self = true
            others = false
            response = fmt.Sprintf("%s not found", arg)

            for k, v := range(users) {
                if v == arg {
                    response = fmt.Sprintf("%s is known as [%s]", k, v)
                }
            }
        case "/whoami":
            self = true
            others = false
            response = fmt.Sprintf("%s is known as [%s]", user, users[user])
        case "/alias":
            self = true
            others = true
            response = fmt.Sprintf("[%s] is now known as [%s]", users[user], arg)
            users[user] = arg
            saveUsers()
        }
    }

    for k, _ := range(users) {
        if (k == user && self) || (k != user && others) {
            client.Send(xmpp.Chat{Remote: k, Type: "chat", Text: response})
        }
    }
}

func loadUsers() {
    users = map[string]string{}

    buf, err := db.Read("users")
    if err != nil {
        log.Println(err)
        return
    }

    emails := strings.Split(string(buf), "\n")
    for e := range(emails) {
        buf, err = db.Read(emails[e])
        if err != nil {
            log.Printf("Could not locate user: %s\n", e)
        } else {
            users[emails[e]] = string(buf)
        }
    }
}

func saveUsers() {
    emails := []string{}
    for k,v := range(users) {
        emails = append(emails, k)
        db.Write(k, []byte(v))
    }

    email_list := strings.Join(emails, "\n")
    db.Write("users", []byte(email_list))
}

func fromUTF8(s string) string {
    ic, err := iconv.Open("char", "UTF-8")
    if err != nil {
        return s
    }
    defer ic.Close()
    ret, _ := ic.Conv(s)
    return ret
}

func toUTF8(s string) string {
    ic, err := iconv.Open("UTF-8", "char")
    if err != nil {
        return s
    }
    defer ic.Close()
    ret, _ := ic.Conv(s)
    return ret
}

func main() {
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "usage: chatty [options]\n")
        flag.PrintDefaults()
        os.Exit(2)
    }
    flag.Parse()

    if *adduser != "" {
        loadUsers()
        users[*adduser] = *adduser
        saveUsers()
        return
    }

    if *rmuser != "" {
        loadUsers()
        delete(users, *adduser)
        saveUsers()
        return
    }


    if *username == "" || *password == "" {
        flag.Usage()
    }

    loadUsers()

    talk, err := xmpp.NewClient(*server, *username, *password)
    if err != nil {
        log.Fatal(err)
    }
    log.Println("Connected.");

    go func() {
        t := time.Tick(1 * time.Minute)
        for _ = range t {
            talk.Present()
        }
    }()

    for {
        chat, err := talk.Recv()
        if err != nil {
            log.Println(err)
        }

        log.Println(chat)
        if chat.Type == "chat" {
            handle(talk, chat)
        }
    }
}
