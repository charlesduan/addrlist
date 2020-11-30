package client

import (
    "git.sr.ht/~sircmpwn/aerc/config"
    "git.sr.ht/~sircmpwn/aerc/lib"
    "git.sr.ht/~sircmpwn/aerc/worker"
    "git.sr.ht/~sircmpwn/aerc/models"
    "git.sr.ht/~sircmpwn/aerc/worker/types"
    "log"
    "github.com/emersion/go-message/mail"
)

type AccountClientCallback func(string, *mail.Address, *models.MessageInfo)

type AccountClient struct {
    acct        *config.AccountConfig
    logger      *log.Logger
    folder      string
    worker      *types.Worker
    store       *lib.MessageStore
    callback    AccountClientCallback
}

func NewAccountClient(
    acct        *config.AccountConfig,
    folder      string,
    logger      *log.Logger,
    callback    AccountClientCallback,
) (*AccountClient, error) {

    client := &AccountClient{
        acct:   acct,
        logger: logger,
        folder: folder,
        callback: callback,
    }

    worker, err := worker.NewWorker(acct.Source, logger)
    if err != nil {
        logger.Printf("%s: %s\n", acct.Name, err)
        return client, err
    }
    client.worker = worker

    go worker.Backend.Run()
    worker.PostAction(&types.Configure{Config: acct}, nil)
    worker.PostAction(&types.Connect{}, nil)
    logger.Printf("Connecting %s...\n", acct.Name)

    return client, nil
}

func (client *AccountClient) Channel() chan types.WorkerMessage {
    return client.worker.Messages
}

func (client *AccountClient) ProcessMessage(msg types.WorkerMessage) bool {
    client.logger.Printf("Got message %T\n", msg)
    msg = client.worker.ProcessMessage(msg)

    switch msg := msg.(type) {
    case *types.Error:
        client.logger.Printf("Response to %T: error %v\n", msg.InResponseTo(),
            msg.Error)
        return false

    case *types.DirectoryInfo:
        client.logger.Printf("Got DirectoryInfo; creating message store\n")
        if client.store == nil {
            client.store = lib.NewMessageStore(client.worker, msg.Info,
                []*types.SortCriterion{ { types.SortArrival, true } },
                nil, nil)
            client.store.Update(msg)
        }

    case *types.DirectoryContents:
        client.logger.Printf("Received DirectoryContents\n")
        if client.store != nil {
            client.store.Update(msg)
            client.store.FetchHeaders(client.store.Uids(), nil)
        }

    case *types.MessageInfo:
        client.parseMessage(msg)

    case *types.Done:
        switch msg.InResponseTo().(type) {
        case *types.Connect:
            return client.connected(msg)
        case *types.OpenDirectory:
            return client.openedDirectory(msg)
        case *types.FetchDirectoryContents:
            return client.fetchedDirContents(msg)
        case *types.FetchMessageHeaders:
            return client.fetchedMsgHeaders(msg)
        default:
            client.logger.Printf("Unexpected Done for %T\n",
                msg, msg.InResponseTo())
        }

    default:
        client.logger.Printf(
            "Unexpected message %T, in response to %T\n",
            msg, msg.InResponseTo(),
        )
    }
    return true
}

func (client *AccountClient) connected(msg *types.Done) bool {
    client.logger.Printf("Connected %s.\n", client.acct.Name)
    client.worker.PostAction(&types.OpenDirectory{
        Directory: client.folder,
    }, nil)
    return true
}

func (client *AccountClient) openedDirectory(msg types.WorkerMessage) bool {
    client.logger.Printf("Opened directory: %T.\n", msg)
    return true
}

func (client *AccountClient) fetchedDirContents(msg types.WorkerMessage) bool {
    client.logger.Printf("fetchedDirContents: %T.\n", msg)
    return true
}

func (client *AccountClient) fetchedMsgHeaders(msg types.WorkerMessage) bool {
    client.logger.Printf("fetchedMsgHeaders: %T.\n", msg)
    return false
}

func (client *AccountClient) parseMessage(msg *types.MessageInfo) {
    envelope := msg.Info.Envelope
    addrLists := []*[]*mail.Address{
        &envelope.From, &envelope.ReplyTo, &envelope.To, &envelope.Cc,
        &envelope.Bcc,
    }

    for _, list := range addrLists {
        for _, addr := range *list {
            client.callback(client.acct.Name, addr, msg.Info)
        }
    }
}

