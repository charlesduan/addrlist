package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "reflect"
    "path"
    "sort"

    "git.sr.ht/~sircmpwn/getopt"
    "git.sr.ht/~sircmpwn/aerc/worker/types"
    "git.sr.ht/~sircmpwn/aerc/config"
    "git.sr.ht/~sircmpwn/aerc/models"

    "github.com/go-ini/ini"
    "github.com/kyoh86/xdg"
    "github.com/emersion/go-message/mail"
    "github.com/mitchellh/go-homedir"

    "github.com/charlesduan/addrlist/client"
    "github.com/charlesduan/addrlist/list"
)

var (
    ShareDir string
    Version  string
    logger   *log.Logger
    al       *list.AddressList
)

func usage() {
    log.Fatal("Usage: addrlist [-vdh] [command]")
}

func readConfig(name string) *ini.File {
    filename := path.Join(xdg.ConfigHome(), "aerc", name)
    logger.Printf("Reading config file %s\n", filename)
    if file, err := ini.Load(filename); err == nil {
        return file
    } else {
        logger.Printf("Failed to read %s file: %v", name, err)
        return nil
    }
}

func readAddrlistDB() string {
    file := readConfig("aerc.conf")
    if file == nil { logger.Fatal("Failed to open config file") }
    if sec := file.Section("general"); sec.HasKey("addrlist-db") {
        val := sec.Key("addrlist-db").String()
        if dbfile, err := homedir.Expand(val); err == nil {
            return dbfile
        } else {
            return val
        }
    } else {
        logger.Fatal("No addrlist-db found in config")
        return ""
    }
}

func readFolderMap() map[string]string {
    file := readConfig("accounts.conf")
    if file == nil { return nil }

    m := make(map[string]string)
    for _, secName := range file.SectionStrings() {
        if secName == "DEFAULT" { continue }
        sec := file.Section(secName)
        if sec.HasKey("address-folder") {
            m[secName] = sec.Key("address-folder").String()
            logger.Printf("Account %s: will read folder %s\n", secName,
                m[secName])
        }
    }
    return m;
}

func clientCallback(
    account string,
    addr *mail.Address,
    info *models.MessageInfo,
) {
    al.ReceiveRecord(addr.Address, addr.Name, info.InternalDate)
}

func parseOpts() bool {
    opts, optind, err := getopt.Getopts(os.Args, "vdh")
    if err != nil {
        log.Print(err)
        usage()
        return false
    }
    for _, opt := range opts {
        switch opt.Option {
        case 'v':
            fmt.Println("addrlist " + Version)
            return false
        case 'd':
            logger = log.New(os.Stdout, "", log.LstdFlags)
        case 'h':
            usage()
            return false
        }
    }

    al = list.NewAddressList(logger)

    args := os.Args[optind:]
    if len(args) > 0 {
        switch args[0] {
        case "find":
            FindMatches(args[1])
            return false
        default:
            usage()
            return false
        }
        return false
    }

    return true
}

func FindMatches(search string) {
    if len(search) < 3 { return }
    if recs, err := al.FindMatches(search, readAddrlistDB(), 20); err == nil {
        sort.Slice(recs, func (i, j int) bool {
            return recs[i].Name < recs[j].Name
        })
        for _, rec := range recs {
            fmt.Printf("%s\t%s\n", rec.Email, rec.Name)
        }
    } else {
        logger.Fatalf("find matches error: %v\n", err)
    }
}


func main() {
    logger = log.New(ioutil.Discard, "", log.LstdFlags)
    if !parseOpts() {
        return
    }

    logger.Println("Starting up aerc")

    // Run aerc's config loader
    conf, err := config.LoadConfigFromFile(nil, ShareDir)
    if err != nil {
        logger.Fatalf("Failed to load config: %v\n", err)
    }

    // Read special config information for addrlist
    addrlistDB := readAddrlistDB()
    folders := readFolderMap()

    // Data storage
    clients := make([]*client.AccountClient, 0, len(conf.Accounts))
    cases := make([]reflect.SelectCase, 0, len(conf.Accounts))

    // For each account, create the AccountClient and the select case. This will
    // initialize a Worker for each account and start the connection.
    for i, _ := range conf.Accounts {
        acct := &conf.Accounts[i]
        logger.Printf("Found account %s\n", acct.Name)
        if (folders[acct.Name] == "") {
            logger.Printf("No folder to read for %s\n", acct.Name)
            continue
        }

        theclient, err := client.NewAccountClient(acct, folders[acct.Name],
            logger, clientCallback)
        if err == nil {
            clients = append(clients, theclient)
            cases = append(cases, reflect.SelectCase{
                Dir: reflect.SelectRecv,
                Chan: reflect.ValueOf(theclient.Channel()),
            })
        } else {
            logger.Printf("Failed to create client for %s\n", acct.Name)
        }
    }


    // Open and read the database file. Since the Workers are all opening
    // connections in parallel, this seems like a good place to do the file
    // reading.
    if err := al.Import(addrlistDB); err != nil {
        logger.Fatalf("import error: %v\n", err)
    }

    // Run the event loop. As the Workers produce results, they will send
    // messages that will be picked up in the Select call. Those messages are
    // then dispatched to the appropriate AccountClient for handling; the
    // AccountClient will use them to continue with its protocol operations.
    for len(cases) > 0 {
        chosen, value, ok := reflect.Select(cases)
        if ok {
            cont := clients[chosen].ProcessMessage(
                value.Interface().(types.WorkerMessage),
            )
            if !cont {
                clients[chosen] = clients[len(clients) - 1]
                clients = clients[:len(clients) - 1]
                cases[chosen] = cases[len(cases) - 1]
                cases = cases[:len(cases) - 1]
            }

        } else {
            logger.Printf("Unexpected channel closure\n")
        }
    }

    if err := al.Export(addrlistDB); err != nil {
        logger.Fatalf("export error: %v\n", err)
    }
}


